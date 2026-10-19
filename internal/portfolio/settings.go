package portfolio

import (
	"errors"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/config"
	"github.com/Bebii-Digital/bebii-blog/internal/cloudflare"
	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

// Settings is the checkout configuration, with env values used when the database row is empty.
type Settings struct {
	Price                   int64
	PaymentWhatsApp         string
	CloudflareAPIToken     string
	CloudflareZoneID        string
	CloudflareRecordType    string
	CloudflareRecordContent string
	CloudflareProxied       bool
	MidtransServerKey       string
	MidtransClientKey       string
	MidtransIsProduction    bool
}

func (s Settings) CloudflareReady() bool {
	return strings.TrimSpace(s.CloudflareAPIToken) != "" &&
		strings.TrimSpace(s.CloudflareZoneID) != "" &&
		strings.TrimSpace(s.CloudflareRecordContent) != ""
}

func (s Settings) DNS() *cloudflare.Client {
	if !s.CloudflareReady() {
		return nil
	}
	return cloudflare.New(s.CloudflareAPIToken, s.CloudflareZoneID, s.CloudflareRecordType, s.CloudflareRecordContent, s.CloudflareProxied)
}

// LoadSettings reads the singleton row. Blank secrets and a zero price fall back to env.
func LoadSettings(db *gorm.DB, cfg config.Config) Settings {
	out := Settings{
		Price:                cfg.PortfolioPrice,
		MidtransServerKey:    cfg.MidtransServerKey,
		MidtransClientKey:    cfg.MidtransClientKey,
		MidtransIsProduction: cfg.MidtransIsProduction,
		CloudflareRecordType: "CNAME",
	}
	if db == nil {
		return out
	}
	var row entity.PortfolioSetting
	err := db.First(&row, 1).Error
	if err != nil {
		return out
	}
	if row.Price > 0 {
		out.Price = row.Price
	}
	out.PaymentWhatsApp = strings.TrimSpace(row.PaymentWhatsApp)
	if token := strings.TrimSpace(row.CloudflareAPIToken); token != "" {
		out.CloudflareAPIToken = token
	}
	if zone := strings.TrimSpace(row.CloudflareZoneID); zone != "" {
		out.CloudflareZoneID = zone
	}
	if kind := strings.TrimSpace(row.CloudflareRecordType); kind != "" {
		out.CloudflareRecordType = strings.ToUpper(kind)
	}
	if content := strings.TrimSpace(row.CloudflareRecordContent); content != "" {
		out.CloudflareRecordContent = content
	}
	out.CloudflareProxied = row.CloudflareProxied
	if key := strings.TrimSpace(row.MidtransServerKey); key != "" {
		out.MidtransServerKey = key
	}
	if key := strings.TrimSpace(row.MidtransClientKey); key != "" {
		out.MidtransClientKey = key
	}
	if row.MidtransServerKey != "" || row.MidtransClientKey != "" {
		out.MidtransIsProduction = row.MidtransIsProduction
	}
	return out
}

// SettingForm is the admin portfolios form.
type SettingForm struct {
	Price                   string
	PaymentWhatsApp         string
	CloudflareAPIToken     string
	CloudflareZoneID        string
	CloudflareRecordType    string
	CloudflareRecordContent string
	CloudflareProxied       bool
	MidtransServerKey        string
	MidtransClientKey        string
	MidtransIsProduction     bool
}

// SaveSettings writes the singleton row. Blank secret fields keep the stored value.
func SaveSettings(db *gorm.DB, form SettingForm) error {
	price, err := ParsePrice(form.Price)
	if err != nil {
		return err
	}
	whatsapp := ""
	if strings.TrimSpace(form.PaymentWhatsApp) != "" {
		if msg := ValidateWhatsApp(form.PaymentWhatsApp); msg != "" {
			return &InputError{Message: msg}
		}
		whatsapp = NormalizeWhatsApp(form.PaymentWhatsApp)
	}
	kind := strings.ToUpper(strings.TrimSpace(form.CloudflareRecordType))
	if kind != "A" && kind != "CNAME" {
		kind = "CNAME"
	}
	token := strings.TrimSpace(form.CloudflareAPIToken)
	zone := strings.TrimSpace(form.CloudflareZoneID)
	content := strings.TrimSpace(form.CloudflareRecordContent)
	if (token != "" || zone != "" || content != "") && (zone == "" || content == "") {
		return &InputError{Message: "Zone ID dan tujuan DNS Cloudflare wajib diisi bersama."}
	}

	var row entity.PortfolioSetting
	err = db.First(&row, 1).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if token == "" {
		token = row.CloudflareAPIToken
	}
	serverKey := strings.TrimSpace(form.MidtransServerKey)
	clientKey := strings.TrimSpace(form.MidtransClientKey)
	if serverKey == "" {
		serverKey = row.MidtransServerKey
	}
	if clientKey == "" {
		clientKey = row.MidtransClientKey
	}
	if (token != "" || zone != "" || content != "") && token == "" {
		return &InputError{Message: "API token Cloudflare wajib diisi."}
	}
	row.ID = 1
	row.Price = price
	row.PaymentWhatsApp = whatsapp
	row.CloudflareAPIToken = token
	row.CloudflareZoneID = zone
	row.CloudflareRecordType = kind
	row.CloudflareRecordContent = content
	row.CloudflareProxied = form.CloudflareProxied
	row.MidtransServerKey = serverKey
	row.MidtransClientKey = clientKey
	row.MidtransIsProduction = form.MidtransIsProduction
	return db.Save(&row).Error
}

// ParsePrice accepts 149000 or 149.000.
func ParsePrice(raw string) (int64, error) {
	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(strings.ToLower(clean), "rp")
	var b strings.Builder
	for _, r := range clean {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return 0, &InputError{Message: "Harga wajib diisi."}
	}
	n, err := strconv.ParseInt(b.String(), 10, 64)
	if err != nil || n <= 0 {
		return 0, &InputError{Message: "Harga tidak valid."}
	}
	return n, nil
}
