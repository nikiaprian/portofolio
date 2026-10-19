package portfolio

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/cloudflare"
	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/midtrans"
)

var (
	ErrOrderNotFound   = errors.New("order not found")
	ErrBadSignature    = errors.New("bad signature")
	ErrAmountMismatch  = errors.New("amount mismatch")
	ErrNotPaid         = errors.New("order is not paid")
	ErrPasswordAlready = errors.New("password already set")
)

// InputError is a message safe to show on the checkout or editor form.
type InputError struct {
	Message string
}

func (e *InputError) Error() string { return e.Message }

// CheckoutInput is the public purchase form.
type CheckoutInput struct {
	Name      string
	Email     string
	WhatsApp  string
	Subdomain string
}

// ProfileInput is the buyer editor form.
type ProfileInput struct {
	DisplayName string
	Headline    string
	Bio         string
	Email       string
	WhatsApp    string
	Links       entity.PortfolioLinks
	Skills      []entity.PortfolioSkill
	Projects    []entity.PortfolioProject
}

// Notification is a Midtrans HTTP notification.
type Notification struct {
	OrderID           string `json:"order_id"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	SignatureKey      string `json:"signature_key"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
	TransactionID     string `json:"transaction_id"`
}

// Service owns portfolio checkout, activation, and buyer edits.
type Service struct {
	db          *gorm.DB
	price       int64
	payments    *midtrans.Client
	payWhatsApp string
	rootDomain  string
	dns         *cloudflare.Client
}

// Options configures a Service from the current admin settings.
type Options struct {
	Price           int64
	Payments        *midtrans.Client
	PaymentWhatsApp string
	RootDomain      string
	DNS             *cloudflare.Client
}

func NewService(db *gorm.DB, opt Options) *Service {
	if opt.Price <= 0 {
		opt.Price = 149000
	}
	return &Service{
		db:          db,
		price:       opt.Price,
		payments:    opt.Payments,
		payWhatsApp: strings.TrimSpace(opt.PaymentWhatsApp),
		rootDomain:  strings.TrimSpace(opt.RootDomain),
		dns:         opt.DNS,
	}
}

func (s *Service) Price() int64 { return s.price }

func (s *Service) Checkout(in CheckoutInput, finishURL func(publicRef string) string) (*entity.PortfolioOrder, error) {
	name := strings.TrimSpace(in.Name)
	email := NormalizeEmail(in.Email)
	whatsapp := NormalizeWhatsApp(in.WhatsApp)
	sub := NormalizeSubdomain(in.Subdomain)
	if msg := ValidatePersonName(name); msg != "" {
		return nil, &InputError{Message: msg}
	}
	if msg := ValidateEmail(email); msg != "" {
		return nil, &InputError{Message: msg}
	}
	if msg := ValidateWhatsApp(in.WhatsApp); msg != "" {
		return nil, &InputError{Message: msg}
	}
	if msg := ValidateSubdomain(sub); msg != "" {
		return nil, &InputError{Message: msg}
	}
	if ValidateWhatsApp(s.payWhatsApp) != "" {
		return nil, &InputError{Message: "Nomor WhatsApp pembayaran belum diatur di admin."}
	}
	fqdn := strings.Trim(sub+"."+s.rootDomain, ".")
	if s.dns != nil {
		taken, dnsErr := s.dns.Exists(fqdn)
		if dnsErr != nil {
			log.Printf("portfolio dns exists: %v", dnsErr)
			return nil, &InputError{Message: "Pengecekan domain gagal. Coba lagi sebentar."}
		}
		if taken {
			return nil, &InputError{Message: "Subdomain ini sudah dipakai."}
		}
	}

	now := time.Now()
	publicRef, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	setupToken, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	order := entity.PortfolioOrder{
		PublicRef:       publicRef,
		SetupToken:      setupToken,
		MidtransOrderID: "PF" + publicRef,
		BuyerName:       name,
		Email:           email,
		WhatsApp:        whatsapp,
		Subdomain:       sub,
		Amount:          s.price,
		Status:          entity.PortfolioOrderPending,
	}
	var stale []entity.Portfolio
	_ = s.db.Where("status = ? AND created_at < ? AND dns_record_id <> ''", entity.PortfolioPending, now.Add(-PendingHold)).Find(&stale).Error
	site := entity.Portfolio{
		Subdomain:   sub,
		Status:      entity.PortfolioPending,
		DisplayName: name,
		Email:       email,
		WhatsApp:    whatsapp,
		Skills:      []entity.PortfolioSkill{},
		Projects:    []entity.PortfolioProject{},
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := releaseStalePending(tx, now); err != nil {
			return err
		}
		if msg, err := buyerEmailAvailable(tx, email, now); err != nil {
			return err
		} else if msg != "" {
			return &InputError{Message: msg}
		}
		if msg, err := subdomainAvailable(tx, sub, now); err != nil {
			return err
		} else if msg != "" {
			return &InputError{Message: msg}
		}
		if err := tx.Create(&site).Error; err != nil {
			if isDuplicate(err) {
				return &InputError{Message: "Subdomain ini baru saja dipesan."}
			}
			return err
		}
		order.PortfolioID = site.ID
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.deleteReleasedDNS(stale)
	if s.dns != nil {
		recordID, dnsErr := s.dns.Create(fqdn)
		if dnsErr != nil {
			log.Printf("portfolio dns create: %v", dnsErr)
			_ = s.db.Where("portfolio_id = ?", site.ID).Delete(&entity.PortfolioOrder{}).Error
			_ = s.db.Delete(&entity.Portfolio{}, site.ID).Error
			return nil, &InputError{Message: "Domain belum bisa ditambahkan. Coba lagi sebentar."}
		}
		if err := s.db.Model(&entity.Portfolio{}).Where("id = ?", site.ID).Update("dns_record_id", recordID).Error; err != nil {
			_ = s.dns.Delete(recordID)
			return nil, err
		}
	}
	_ = finishURL
	return &order, nil
}

func (s *Service) deleteReleasedDNS(stale []entity.Portfolio) {
	if s.dns == nil {
		return
	}
	for _, site := range stale {
		var n int64
		if err := s.db.Model(&entity.Portfolio{}).Where("id = ?", site.ID).Count(&n).Error; err != nil || n > 0 {
			continue
		}
		if err := s.dns.Delete(site.DNSRecordID); err != nil {
			log.Printf("portfolio dns delete: %v", err)
		}
	}
}

func (s *Service) OrderByPublicRef(ref string) (*entity.PortfolioOrder, error) {
	var order entity.PortfolioOrder
	err := s.db.Where("public_ref = ?", strings.TrimSpace(ref)).First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *Service) OrderBySetupToken(token string) (*entity.PortfolioOrder, error) {
	var order entity.PortfolioOrder
	err := s.db.Where("setup_token = ?", strings.TrimSpace(token)).First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *Service) PortfolioByID(id uint) (*entity.Portfolio, error) {
	var site entity.Portfolio
	if err := s.db.First(&site, id).Error; err != nil {
		return nil, err
	}
	return &site, nil
}

func (s *Service) ActiveBySubdomain(sub string) (*entity.Portfolio, error) {
	var site entity.Portfolio
	err := s.db.Where("subdomain = ? AND status = ?", NormalizeSubdomain(sub), entity.PortfolioActive).First(&site).Error
	if err != nil {
		return nil, err
	}
	return &site, nil
}

func (s *Service) HandleNotification(n Notification) error {
	if s.payments == nil || !midtrans.VerifySignature(n.OrderID, n.StatusCode, n.GrossAmount, s.payments.ServerKey(), n.SignatureKey) {
		return ErrBadSignature
	}
	var order entity.PortfolioOrder
	err := s.db.Where("midtrans_order_id = ?", strings.TrimSpace(n.OrderID)).First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrOrderNotFound
	}
	if err != nil {
		return err
	}
	if !amountMatches(order.Amount, n.GrossAmount) {
		return ErrAmountMismatch
	}
	status := strings.ToLower(strings.TrimSpace(n.TransactionStatus))
	fraud := strings.ToLower(strings.TrimSpace(n.FraudStatus))
	paid := status == "settlement" || (status == "capture" && (fraud == "" || fraud == "accept"))
	if order.Status == entity.PortfolioOrderPaid {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"transaction_status": status}
		switch {
		case paid:
			updates["status"] = entity.PortfolioOrderPaid
		case status == "deny":
			updates["status"] = entity.PortfolioOrderDeny
		case status == "cancel":
			updates["status"] = entity.PortfolioOrderCancel
		case status == "expire":
			updates["status"] = entity.PortfolioOrderExpire
		}
		if err := tx.Model(&entity.PortfolioOrder{}).Where("id = ?", order.ID).Updates(updates).Error; err != nil {
			return err
		}
		if !paid {
			return nil
		}
		if err := tx.Model(&entity.Portfolio{}).Where("id = ?", order.PortfolioID).Update("status", entity.PortfolioActive).Error; err != nil {
			return err
		}
		var existing entity.PortfolioAccount
		err := tx.Where("portfolio_id = ?", order.PortfolioID).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&entity.PortfolioAccount{
				PortfolioID: order.PortfolioID,
				Email:       order.Email,
			}).Error
		}
		return err
	})
}

func (s *Service) SetPassword(token, password string) error {
	if len(password) < 8 || len(password) > 72 {
		return &InputError{Message: "Password harus 8 sampai 72 karakter."}
	}
	order, err := s.OrderBySetupToken(token)
	if err != nil {
		return err
	}
	if order.Status != entity.PortfolioOrderPaid {
		return ErrNotPaid
	}
	var account entity.PortfolioAccount
	if err := s.db.Where("portfolio_id = ?", order.PortfolioID).First(&account).Error; err != nil {
		return err
	}
	if strings.TrimSpace(account.Password) != "" {
		return ErrPasswordAlready
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.Model(&account).Update("password", string(hashed)).Error
}

func (s *Service) Login(email, password string) (*entity.PortfolioAccount, error) {
	email = NormalizeEmail(email)
	var account entity.PortfolioAccount
	err := s.db.Where("email = ?", email).First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &InputError{Message: "Email atau password salah."}
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(account.Password) == "" {
		return nil, &InputError{Message: "Password belum diatur. Gunakan tautan dari halaman selesai pembayaran."}
	}
	if bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(password)) != nil {
		return nil, &InputError{Message: "Email atau password salah."}
	}
	return &account, nil
}

func (s *Service) Account(id uint) (*entity.PortfolioAccount, *entity.Portfolio, error) {
	var account entity.PortfolioAccount
	if err := s.db.First(&account, id).Error; err != nil {
		return nil, nil, err
	}
	site, err := s.PortfolioByID(account.PortfolioID)
	if err != nil {
		return nil, nil, err
	}
	return &account, site, nil
}

func (s *Service) UpdateProfile(portfolioID uint, in ProfileInput) error {
	name := strings.TrimSpace(in.DisplayName)
	if msg := ValidatePersonName(name); msg != "" {
		return &InputError{Message: msg}
	}
	headline := trimRunes(in.Headline, 160)
	bio := trimRunes(in.Bio, 2000)
	email := ""
	if strings.TrimSpace(in.Email) != "" {
		if msg := ValidateEmail(in.Email); msg != "" {
			return &InputError{Message: msg}
		}
		email = NormalizeEmail(in.Email)
	}
	whatsapp := ""
	if strings.TrimSpace(in.WhatsApp) != "" {
		if msg := ValidateWhatsApp(in.WhatsApp); msg != "" {
			return &InputError{Message: msg}
		}
		whatsapp = NormalizeWhatsApp(in.WhatsApp)
	}
	var site entity.Portfolio
	if err := s.db.First(&site, portfolioID).Error; err != nil {
		return err
	}
	site.DisplayName = name
	site.Headline = headline
	site.Bio = bio
	site.Email = email
	site.WhatsApp = whatsapp
	site.Links = entity.PortfolioLinks{
		Website:   SafeHTTPURL(in.Links.Website),
		Instagram: SafeHTTPURL(in.Links.Instagram),
		LinkedIn:  SafeHTTPURL(in.Links.LinkedIn),
		GitHub:    SafeHTTPURL(in.Links.GitHub),
	}
	site.Skills = in.Skills
	if site.Skills == nil {
		site.Skills = []entity.PortfolioSkill{}
	}
	site.Projects = in.Projects
	if site.Projects == nil {
		site.Projects = []entity.PortfolioProject{}
	}
	return s.db.Save(&site).Error
}

func (s *Service) SetPhoto(portfolioID uint, photoURL string) error {
	return s.db.Model(&entity.Portfolio{}).Where("id = ?", portfolioID).Update("photo_url", photoURL).Error
}

func (s *Service) SetStatus(portfolioID uint, status string) error {
	if status != entity.PortfolioActive && status != entity.PortfolioSuspended {
		return &InputError{Message: "Status tidak dikenal."}
	}
	var site entity.Portfolio
	if err := s.db.First(&site, portfolioID).Error; err != nil {
		return err
	}
	if site.Status == entity.PortfolioPending {
		return &InputError{Message: "Portfolio yang belum lunas tidak bisa diubah statusnya."}
	}
	return s.db.Model(&site).Update("status", status).Error
}

func (s *Service) AdminLists() ([]entity.Portfolio, []entity.PortfolioOrder, error) {
	var sites []entity.Portfolio
	if err := s.db.Order("id desc").Limit(200).Find(&sites).Error; err != nil {
		return nil, nil, err
	}
	var orders []entity.PortfolioOrder
	if err := s.db.Order("id desc").Limit(200).Find(&orders).Error; err != nil {
		return nil, nil, err
	}
	return sites, orders, nil
}

func (s *Service) PasswordSet(portfolioID uint) (bool, error) {
	var account entity.PortfolioAccount
	err := s.db.Where("portfolio_id = ?", portfolioID).First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(account.Password) != "", nil
}

func releaseStalePending(tx *gorm.DB, now time.Time) error {
	cutoff := now.Add(-PendingHold)
	var stale []entity.Portfolio
	if err := tx.Where("status = ? AND created_at < ?", entity.PortfolioPending, cutoff).Find(&stale).Error; err != nil {
		return err
	}
	for _, site := range stale {
		var paid int64
		if err := tx.Model(&entity.PortfolioOrder{}).Where("portfolio_id = ? AND status = ?", site.ID, entity.PortfolioOrderPaid).Count(&paid).Error; err != nil {
			return err
		}
		if paid > 0 {
			continue
		}
		if err := tx.Where("portfolio_id = ?", site.ID).Delete(&entity.PortfolioOrder{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&entity.Portfolio{}, site.ID).Error; err != nil {
			return err
		}
	}
	return nil
}

func buyerEmailAvailable(tx *gorm.DB, email string, now time.Time) (string, error) {
	var account entity.PortfolioAccount
	err := tx.Where("email = ?", email).First(&account).Error
	if err == nil {
		return "Email ini sudah dipakai.", nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	var pending entity.Portfolio
	err = tx.Where("email = ? AND status = ? AND created_at > ?", email, entity.PortfolioPending, now.Add(-PendingHold)).First(&pending).Error
	if err == nil {
		return "Email ini sedang menunggu pembayaran.", nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	return "", nil
}

func subdomainAvailable(tx *gorm.DB, sub string, now time.Time) (string, error) {
	var site entity.Portfolio
	err := tx.Where("subdomain = ?", sub).First(&site).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if SubdomainHeld(site.Status, site.CreatedAt, now) {
		return "Subdomain ini sudah dipakai.", nil
	}
	var paid int64
	if err := tx.Model(&entity.PortfolioOrder{}).Where("portfolio_id = ? AND status = ?", site.ID, entity.PortfolioOrderPaid).Count(&paid).Error; err != nil {
		return "", err
	}
	if paid > 0 {
		return "Subdomain ini sudah dipakai.", nil
	}
	if err := tx.Where("portfolio_id = ?", site.ID).Delete(&entity.PortfolioOrder{}).Error; err != nil {
		return "", err
	}
	if err := tx.Delete(&entity.Portfolio{}, site.ID).Error; err != nil {
		return "", err
	}
	return "", nil
}

func amountMatches(expected int64, gross string) bool {
	value, err := strconv.ParseFloat(strings.TrimSpace(gross), 64)
	if err != nil {
		return false
	}
	diff := value - float64(expected)
	if diff < 0 {
		diff = -diff
	}
	return diff < 0.001
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func trimRunes(raw string, max int) string {
	raw = strings.TrimSpace(raw)
	r := []rune(raw)
	if len(r) > max {
		return string(r[:max])
	}
	return raw
}

func isDuplicate(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique")
}
