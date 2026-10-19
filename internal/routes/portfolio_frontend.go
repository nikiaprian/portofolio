package routes

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/config"
	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/post"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/seo"
	"github.com/Bebii-Digital/bebii-blog/internal/midtrans"
	"github.com/Bebii-Digital/bebii-blog/internal/portfolio"
)

func portfolioSvc(db *gorm.DB, cfg config.Config) *portfolio.Service {
	client := midtrans.NewClient(cfg.MidtransServerKey, cfg.MidtransClientKey, cfg.MidtransIsProduction)
	return portfolio.NewService(db, cfg.PortfolioPrice, client)
}

// PortfolioHostMiddleware serves an active buyer site on {name}.{root} and leaves the apex site alone.
func PortfolioHostMiddleware(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		kind, sub := portfolio.ClassifyHost(c.Request.Host, cfg.PortfolioRootDomain)
		switch kind {
		case portfolio.HostApex:
			c.Next()
		case portfolio.HostReserved:
			c.String(http.StatusNotFound, "not found")
			c.Abort()
		default:
			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/static/") || path == "/img" || strings.HasPrefix(path, "/img/") {
				c.Next()
				return
			}
			if (c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead) && (path == "/" || path == "") {
				if err := renderPortfolioPublic(c, db, cfg, sub); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
				}
				c.Abort()
				return
			}
			c.String(http.StatusNotFound, "not found")
			c.Abort()
		}
	}
}

func registerPortfolioBuyerRoutes(r *gin.Engine, db *gorm.DB, cfg config.Config) {
	r.GET("/testimoni", func(c *gin.Context) {
		setting := loadSiteSetting(db)
		if err := renderPortfolioFile(c, http.StatusOK, "testimoni.html", gin.H{
			"layout":   "home",
			"Title":    "Testimoni",
			"SiteName": portfolioSiteName(setting),
			"Favicon":  strings.TrimSpace(setting.FaviconURL),
			"Quotes":   sampleTestimonials(),
		}); err != nil {
			c.String(http.StatusInternalServerError, "render error: %v", err)
		}
	})
	r.GET("/beli", func(c *gin.Context) {
		renderPortfolioCheckout(c, db, cfg, checkoutFields{}, "")
	})
	r.POST("/beli", func(c *gin.Context) {
		fields := checkoutFields{
			Name:      c.PostForm("name"),
			Email:     c.PostForm("email"),
			WhatsApp:  c.PostForm("whatsapp"),
			Subdomain: c.PostForm("subdomain"),
		}
		order, err := portfolioSvc(db, cfg).Checkout(portfolio.CheckoutInput{
			Name:      fields.Name,
			Email:     fields.Email,
			WhatsApp:  fields.WhatsApp,
			Subdomain: fields.Subdomain,
		}, func(ref string) string {
			return strings.TrimRight(seo.RequestOrigin(c), "/") + "/beli/selesai/" + ref
		})
		if err != nil {
			msg := "Pembelian gagal disimpan."
			var inputErr *portfolio.InputError
			if errors.As(err, &inputErr) {
				msg = inputErr.Message
			}
			renderPortfolioCheckout(c, db, cfg, fields, msg)
			return
		}
		c.Redirect(http.StatusSeeOther, "/beli/bayar/"+order.PublicRef)
	})
	r.GET("/beli/bayar/:ref", func(c *gin.Context) {
		renderPortfolioPay(c, db, cfg, c.Param("ref"))
	})
	r.GET("/beli/selesai/:ref", func(c *gin.Context) {
		renderPortfolioDone(c, db, cfg, c.Param("ref"))
	})

	r.GET("/kelola/masuk", func(c *gin.Context) {
		if _, _, ok := portfolioBuyer(c, db, cfg); ok {
			c.Redirect(http.StatusFound, "/kelola")
			return
		}
		notice := ""
		if c.Query("sudah") == "1" {
			notice = "Password tersimpan. Silakan masuk."
		}
		renderPortfolioLogin(c, db, "", notice, "")
	})
	r.POST("/kelola/masuk", func(c *gin.Context) {
		account, err := portfolioSvc(db, cfg).Login(c.PostForm("email"), c.PostForm("password"))
		if err != nil {
			msg := "Email atau password salah."
			var inputErr *portfolio.InputError
			if errors.As(err, &inputErr) {
				msg = inputErr.Message
			}
			renderPortfolioLogin(c, db, c.PostForm("email"), "", msg)
			return
		}
		if err := setPortfolioSession(c, cfg, account.ID); err != nil {
			renderPortfolioLogin(c, db, c.PostForm("email"), "", "Sesi gagal dibuat.")
			return
		}
		c.Redirect(http.StatusSeeOther, "/kelola")
	})
	r.POST("/kelola/keluar", func(c *gin.Context) {
		clearPortfolioSession(c)
		c.Redirect(http.StatusSeeOther, "/kelola/masuk")
	})
	r.GET("/kelola/atur-password/:token", func(c *gin.Context) {
		renderPortfolioPassword(c, db, cfg, c.Param("token"), "")
	})
	r.POST("/kelola/atur-password/:token", func(c *gin.Context) {
		token := c.Param("token")
		err := portfolioSvc(db, cfg).SetPassword(token, c.PostForm("password"))
		if err != nil {
			if errors.Is(err, portfolio.ErrPasswordAlready) {
				renderPortfolioPassword(c, db, cfg, token, "")
				return
			}
			msg := "Password gagal disimpan."
			var inputErr *portfolio.InputError
			if errors.As(err, &inputErr) {
				msg = inputErr.Message
			} else if errors.Is(err, portfolio.ErrNotPaid) {
				msg = "Pembayaran belum lunas."
			} else if errors.Is(err, portfolio.ErrOrderNotFound) {
				c.String(http.StatusNotFound, "tautan tidak ditemukan")
				return
			}
			renderPortfolioPassword(c, db, cfg, token, msg)
			return
		}
		c.Redirect(http.StatusSeeOther, "/kelola/masuk?sudah=1")
	})
	r.GET("/kelola", func(c *gin.Context) {
		_, site, ok := portfolioBuyer(c, db, cfg)
		if !ok {
			c.Redirect(http.StatusFound, "/kelola/masuk")
			return
		}
		renderPortfolioManage(c, db, cfg, site, "", c.Query("tersimpan") == "1")
	})
	r.POST("/kelola", func(c *gin.Context) {
		_, site, ok := portfolioBuyer(c, db, cfg)
		if !ok {
			c.Redirect(http.StatusFound, "/kelola/masuk")
			return
		}
		err := portfolioSvc(db, cfg).UpdateProfile(site.ID, portfolio.ProfileInput{
			DisplayName: c.PostForm("display_name"),
			Headline:    c.PostForm("headline"),
			Bio:         c.PostForm("bio"),
			Email:       c.PostForm("email"),
			WhatsApp:    c.PostForm("whatsapp"),
			Links: entity.PortfolioLinks{
				Website:   withHTTPScheme(c.PostForm("website")),
				Instagram: withHTTPScheme(c.PostForm("instagram")),
				LinkedIn:  withHTTPScheme(c.PostForm("linkedin")),
				GitHub:    withHTTPScheme(c.PostForm("github")),
			},
			Skills:   parseSkillLines(c.PostForm("skills")),
			Projects: parseProjectSlots(c),
		})
		if err != nil {
			msg := "Isi gagal disimpan."
			var inputErr *portfolio.InputError
			if errors.As(err, &inputErr) {
				msg = inputErr.Message
			}
			draft := *site
			draft.DisplayName = c.PostForm("display_name")
			draft.Headline = c.PostForm("headline")
			draft.Bio = c.PostForm("bio")
			draft.Email = c.PostForm("email")
			draft.WhatsApp = c.PostForm("whatsapp")
			draft.Links = entity.PortfolioLinks{
				Website:   c.PostForm("website"),
				Instagram: c.PostForm("instagram"),
				LinkedIn:  c.PostForm("linkedin"),
				GitHub:    c.PostForm("github"),
			}
			draft.Skills = parseSkillLines(c.PostForm("skills"))
			draft.Projects = parseProjectSlots(c)
			renderPortfolioManage(c, db, cfg, &draft, msg, false)
			return
		}
		c.Redirect(http.StatusSeeOther, "/kelola?tersimpan=1")
	})
	r.POST("/kelola/foto", func(c *gin.Context) {
		_, site, ok := portfolioBuyer(c, db, cfg)
		if !ok {
			c.Redirect(http.StatusFound, "/kelola/masuk")
			return
		}
		photoURL, err := savePortfolioPhoto(c, site)
		if err != nil {
			renderPortfolioManage(c, db, cfg, site, err.Error(), false)
			return
		}
		if err := portfolioSvc(db, cfg).SetPhoto(site.ID, photoURL); err != nil {
			renderPortfolioManage(c, db, cfg, site, "Foto gagal disimpan.", false)
			return
		}
		removePortfolioPhoto(site.PhotoURL)
		c.Redirect(http.StatusSeeOther, "/kelola?tersimpan=1")
	})
}

func registerPortfolioAdminRoutes(adminOnly *gin.RouterGroup, db *gorm.DB, cfg config.Config) {
	adminOnly.GET("/portfolios", func(c *gin.Context) {
		renderPortfolioAdmin(c, db, cfg, c.Query("error"))
	})
	adminOnly.POST("/portfolios/:id/status", func(c *gin.Context) {
		id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id64 == 0 {
			c.Redirect(http.StatusSeeOther, "/admin/portfolios?error=Portfolio+tidak+ditemukan")
			return
		}
		err = portfolioSvc(db, cfg).SetStatus(uint(id64), c.PostForm("status"))
		if err != nil {
			msg := "Status gagal diubah."
			var inputErr *portfolio.InputError
			if errors.As(err, &inputErr) {
				msg = inputErr.Message
			}
			c.Redirect(http.StatusSeeOther, "/admin/portfolios?error="+template.URLQueryEscaper(msg))
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin/portfolios")
	})
}

func portfolioMidtransNotification(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var note portfolio.Notification
		if err := c.ShouldBindJSON(&note); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification"})
			return
		}
		err := portfolioSvc(db, cfg).HandleNotification(note)
		switch {
		case err == nil:
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		case errors.Is(err, portfolio.ErrBadSignature):
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid signature"})
		case errors.Is(err, portfolio.ErrOrderNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		case errors.Is(err, portfolio.ErrAmountMismatch):
			c.JSON(http.StatusBadRequest, gin.H{"error": "amount mismatch"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed"})
		}
	}
}

func sampleTestimonials() []gin.H {
	return []gin.H{
		{"Name": "Rina Putri", "Role": "Desainer grafis, Bandung", "Quote": "Alamatnya saya taruh di bio Instagram. Klien langsung buka, tidak perlu saya kirim PDF lagi."},
		{"Name": "Dimas Arya", "Role": "Analis data, Jakarta", "Quote": "Malam itu saya isi keahlian dan dua proyek. Besok paginya tautannya sudah saya lampirkan di lamaran."},
		{"Name": "Sari Wulandari", "Role": "Fotografer, Yogyakarta", "Quote": "Foto dan daftar proyeknya rapi. Hosting tidak perlu saya urus sendiri."},
		{"Name": "Andi Pratama", "Role": "Mahasiswa, Surabaya", "Quote": "Sekali bayar. Password saya atur sendiri, bio dan kontak bisa diubah kapan saja."},
		{"Name": "Lala Kusuma", "Role": "Penulis lepas, Denpasar", "Quote": "Lebih gampang dikirim ke klien daripada folder yang isinya campur aduk."},
		{"Name": "Bima Santoso", "Role": "Product designer, Tangerang", "Quote": "Namanya pendek, jadi enak saya sebut saat wawancara."},
	}
}

func renderPortfolioLanding(c *gin.Context, db *gorm.DB, cfg config.Config) error {
	setting := loadSiteSetting(db)
	demoURL := ""
	var demo entity.Portfolio
	if err := db.Where("subdomain = ? AND status = ?", "contoh", entity.PortfolioActive).First(&demo).Error; err == nil {
		demoURL = portfolio.TenantPublicURL(c, cfg.PortfolioRootDomain, "contoh")
	}
	name := portfolioSiteName(setting)
	title := name + " — Portfolio online"
	if strings.EqualFold(name, "Portfolio") {
		title = "Portfolio online"
	}
	return renderPortfolioFile(c, http.StatusOK, "landing.html", gin.H{
		"layout":      "home",
		"Title":       title,
		"Description": "Beli portfolio online dan dapat subdomain untuk menampilkan foto, bio, keahlian, proyek, dan kontak.",
		"SiteName":    name,
		"Favicon":     strings.TrimSpace(setting.FaviconURL),
		"Root":        cfg.PortfolioRootDomain,
		"PriceLabel":  portfolio.FormatIDR(cfg.PortfolioPrice),
		"DemoURL":     demoURL,
		"Articles":    portfolioLandingArticles(db, setting),
		"Quotes":      sampleTestimonials()[:3],
	})
}

type checkoutFields struct {
	Name, Email, WhatsApp, Subdomain string
}

func renderPortfolioCheckout(c *gin.Context, db *gorm.DB, cfg config.Config, fields checkoutFields, message string) {
	setting := loadSiteSetting(db)
	status := http.StatusOK
	if message != "" {
		status = http.StatusUnprocessableEntity
	}
	c.Header("Cache-Control", "no-store")
	if err := renderPortfolioFile(c, status, "checkout.html", gin.H{
		"Title":      "Beli portfolio",
		"NoIndex":    true,
		"SiteName":   portfolioSiteName(setting),
		"Favicon":    strings.TrimSpace(setting.FaviconURL),
		"Root":       cfg.PortfolioRootDomain,
		"PriceLabel": portfolio.FormatIDR(cfg.PortfolioPrice),
		"Name":       fields.Name,
		"Email":      fields.Email,
		"WhatsApp":   fields.WhatsApp,
		"Subdomain":  fields.Subdomain,
		"Error":      message,
	}); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

func renderPortfolioPay(c *gin.Context, db *gorm.DB, cfg config.Config, ref string) {
	c.Header("Cache-Control", "no-store")
	order, err := portfolioSvc(db, cfg).OrderByPublicRef(ref)
	if err != nil {
		c.String(http.StatusNotFound, "pesanan tidak ditemukan")
		return
	}
	if order.Status == entity.PortfolioOrderPaid {
		c.Redirect(http.StatusFound, "/beli/selesai/"+order.PublicRef)
		return
	}
	client := midtrans.NewClient(cfg.MidtransServerKey, cfg.MidtransClientKey, cfg.MidtransIsProduction)
	if strings.TrimSpace(order.SnapToken) == "" || !client.Configured() {
		c.String(http.StatusBadGateway, "pembayaran tidak tersedia")
		return
	}
	if err := renderPortfolioFile(c, http.StatusOK, "pay.html", gin.H{
		"Title":      "Bayar portfolio",
		"NoIndex":    true,
		"SiteName":   portfolioSiteName(loadSiteSetting(db)),
		"Subdomain":  order.Subdomain,
		"Root":       cfg.PortfolioRootDomain,
		"PriceLabel": portfolio.FormatIDR(order.Amount),
		"SnapJS":     client.SnapJSURL(),
		"ClientKey":  client.ClientKey(),
		"SnapToken":  order.SnapToken,
		"DoneURL":    strings.TrimRight(seo.RequestOrigin(c), "/") + "/beli/selesai/" + order.PublicRef,
	}); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

func renderPortfolioDone(c *gin.Context, db *gorm.DB, cfg config.Config, ref string) {
	c.Header("Cache-Control", "no-store")
	order, err := portfolioSvc(db, cfg).OrderByPublicRef(ref)
	if err != nil {
		c.String(http.StatusNotFound, "pesanan tidak ditemukan")
		return
	}
	setting := loadSiteSetting(db)
	paid := order.Status == entity.PortfolioOrderPaid
	passwordSet := false
	if paid {
		passwordSet, _ = portfolioSvc(db, cfg).PasswordSet(order.PortfolioID)
	}
	if err := renderPortfolioFile(c, http.StatusOK, "done.html", gin.H{
		"Title":       "Status pembayaran",
		"NoIndex":     true,
		"Refresh":     !paid,
		"SiteName":    portfolioSiteName(setting),
		"Paid":        paid,
		"SiteURL":     portfolio.TenantPublicURL(c, cfg.PortfolioRootDomain, order.Subdomain),
		"HostLabel":   order.Subdomain + "." + cfg.PortfolioRootDomain,
		"SetupURL":    "/kelola/atur-password/" + order.SetupToken,
		"PayURL":      "/beli/bayar/" + order.PublicRef,
		"PasswordSet": passwordSet,
	}); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

func renderPortfolioPublic(c *gin.Context, db *gorm.DB, cfg config.Config, sub string) error {
	site, err := portfolioSvc(db, cfg).ActiveBySubdomain(sub)
	if err != nil {
		c.Header("Cache-Control", "no-store")
		return renderPortfolioFile(c, http.StatusNotFound, "missing.html", gin.H{
			"Title": "Portfolio tidak ditemukan",
			"Nav":   "none",
			"Host":  sub + "." + cfg.PortfolioRootDomain,
		})
	}
	projects := make([]gin.H, 0, len(site.Projects))
	for _, project := range site.Projects {
		if strings.TrimSpace(project.Title) == "" {
			continue
		}
		projects = append(projects, gin.H{
			"Title":   project.Title,
			"Summary": project.Summary,
			"URL":     portfolio.SafeHTTPURL(project.URL),
		})
	}
	skills := make([]entity.PortfolioSkill, 0, len(site.Skills))
	for _, skill := range site.Skills {
		if strings.TrimSpace(skill.Name) != "" {
			skills = append(skills, skill)
		}
	}
	return renderPortfolioFile(c, http.StatusOK, "public.html", gin.H{
		"Nav":         "none",
		"DisplayName": site.DisplayName,
		"Headline":    site.Headline,
		"Bio":         site.Bio,
		"PhotoURL":    site.PhotoURL,
		"Initials":    portfolio.Initials(site.DisplayName),
		"Email":       site.Email,
		"WhatsAppURL": portfolio.WhatsAppLink(site.WhatsApp),
		"Skills":      skills,
		"Projects":    projects,
		"Website":     portfolio.SafeHTTPURL(site.Links.Website),
		"Instagram":   portfolio.SafeHTTPURL(site.Links.Instagram),
		"LinkedIn":    portfolio.SafeHTTPURL(site.Links.LinkedIn),
		"GitHub":      portfolio.SafeHTTPURL(site.Links.GitHub),
	})
}

func renderPortfolioLogin(c *gin.Context, db *gorm.DB, email, notice, message string) {
	c.Header("Cache-Control", "no-store")
	status := http.StatusOK
	if message != "" {
		status = http.StatusUnauthorized
	}
	if err := renderPortfolioFile(c, status, "login.html", gin.H{
		"Title":    "Masuk kelola portfolio",
		"NoIndex":  true,
		"SiteName": portfolioSiteName(loadSiteSetting(db)),
		"Email":    email,
		"Notice":   notice,
		"Error":    message,
	}); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

func renderPortfolioPassword(c *gin.Context, db *gorm.DB, cfg config.Config, token, message string) {
	c.Header("Cache-Control", "no-store")
	order, err := portfolioSvc(db, cfg).OrderBySetupToken(token)
	if err != nil {
		c.String(http.StatusNotFound, "tautan tidak ditemukan")
		return
	}
	already := false
	if order.Status == entity.PortfolioOrderPaid {
		already, _ = portfolioSvc(db, cfg).PasswordSet(order.PortfolioID)
	}
	status := http.StatusOK
	if message != "" {
		status = http.StatusUnprocessableEntity
	}
	if err := renderPortfolioFile(c, status, "password.html", gin.H{
		"Title":    "Atur password",
		"NoIndex":  true,
		"SiteName": portfolioSiteName(loadSiteSetting(db)),
		"Token":    token,
		"Error":    message,
		"Already":  already,
	}); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

type projectSlot struct {
	Index   int
	Title   string
	Summary string
	URL     string
}

func renderPortfolioManage(c *gin.Context, db *gorm.DB, cfg config.Config, site *entity.Portfolio, message string, saved bool) {
	c.Header("Cache-Control", "no-store")
	slots := make([]projectSlot, 4)
	for i := range slots {
		slots[i].Index = i
		if i < len(site.Projects) {
			slots[i].Title = site.Projects[i].Title
			slots[i].Summary = site.Projects[i].Summary
			slots[i].URL = site.Projects[i].URL
		}
	}
	var skills strings.Builder
	for i, skill := range site.Skills {
		if i > 0 {
			skills.WriteByte('\n')
		}
		skills.WriteString(skill.Name)
	}
	status := http.StatusOK
	if message != "" {
		status = http.StatusUnprocessableEntity
	}
	if err := renderPortfolioFile(c, status, "manage.html", gin.H{
		"Title":        "Kelola portfolio",
		"NoIndex":      true,
		"Nav":          "bare",
		"SiteURL":      portfolio.TenantPublicURL(c, cfg.PortfolioRootDomain, site.Subdomain),
		"HostLabel":    site.Subdomain + "." + cfg.PortfolioRootDomain,
		"Status":       site.Status,
		"Saved":        saved,
		"Error":        message,
		"PhotoURL":     site.PhotoURL,
		"DisplayName":  site.DisplayName,
		"Headline":     site.Headline,
		"Bio":          site.Bio,
		"SkillsText":   skills.String(),
		"Email":        site.Email,
		"WhatsApp":     site.WhatsApp,
		"Website":      site.Links.Website,
		"Instagram":    site.Links.Instagram,
		"LinkedIn":     site.Links.LinkedIn,
		"GitHub":       site.Links.GitHub,
		"ProjectSlots": slots,
	}); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

func renderPortfolioAdmin(c *gin.Context, db *gorm.DB, cfg config.Config, message string) {
	sites, orders, err := portfolioSvc(db, cfg).AdminLists()
	if err != nil {
		c.String(http.StatusInternalServerError, "portfolio: %v", err)
		return
	}
	loc := portfolioLocation()
	siteRows := make([]gin.H, 0, len(sites))
	for _, site := range sites {
		siteRows = append(siteRows, gin.H{
			"ID":          site.ID,
			"Subdomain":   site.Subdomain,
			"DisplayName": site.DisplayName,
			"Status":      site.Status,
			"Created":     site.CreatedAt.In(loc).Format("02 Jan 2006 15:04"),
		})
	}
	orderRows := make([]gin.H, 0, len(orders))
	for _, order := range orders {
		orderRows = append(orderRows, gin.H{
			"Subdomain": order.Subdomain,
			"BuyerName": order.BuyerName,
			"Email":     order.Email,
			"Amount":    portfolio.FormatIDR(order.Amount),
			"Status":    order.Status,
			"Created":   order.CreatedAt.In(loc).Format("02 Jan 2006 15:04"),
		})
	}
	tmpl, err := template.ParseFiles(
		"public/templates/layouts/admin/app.html",
		"public/templates/layouts/admin/navbar.html",
		"public/templates/layouts/admin/sidebar.html",
		"public/templates/pages/admin/portfolio/index.html",
	)
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	data := adminSitePage(db, "Portfolio", "portfolios", nil)
	data["sites"] = siteRows
	data["orders"] = orderRows
	data["error"] = message
	if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

func renderPortfolioFile(c *gin.Context, status int, name string, data any) error {
	tmpl, err := template.ParseFiles(
		"public/templates/layouts/portfolio/app.html",
		"public/templates/pages/portfolio/"+name,
	)
	if err != nil {
		return err
	}
	if status > 0 {
		c.Status(status)
	}
	return tmpl.ExecuteTemplate(c.Writer, "pf-app", data)
}

func portfolioLandingArticles(db *gorm.DB, setting entity.SiteSetting) []gin.H {
	var posts []entity.Post
	if err := db.Preload("Category").
		Scopes(postutil.ScopeVisiblePublished("", time.Now())).
		Order("created_at desc").
		Limit(4).
		Find(&posts).Error; err != nil {
		return nil
	}
	loc := portfolioLocation()
	months := [...]string{"", "Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
	out := make([]gin.H, 0, len(posts))
	for _, post := range posts {
		created := post.CreatedAt.In(loc)
		label := ""
		if int(created.Month()) < len(months) {
			label = fmt.Sprintf("%d %s %d", created.Day(), months[created.Month()], created.Year())
		}
		out = append(out, gin.H{
			"Title": post.Title,
			"Href":  clientPostPublicPath(setting, post),
			"Date":  label,
		})
	}
	return out
}

func portfolioBuyer(c *gin.Context, db *gorm.DB, cfg config.Config) (*entity.PortfolioAccount, *entity.Portfolio, bool) {
	token, err := c.Cookie(portfolio.SessionCookieName())
	if err != nil || strings.TrimSpace(token) == "" {
		return nil, nil, false
	}
	id, err := portfolio.ParseSession(cfg.JWTSecret, token)
	if err != nil {
		return nil, nil, false
	}
	account, site, err := portfolioSvc(db, cfg).Account(id)
	if err != nil {
		return nil, nil, false
	}
	return account, site, true
}

func setPortfolioSession(c *gin.Context, cfg config.Config, accountID uint) error {
	hours := cfg.JWTExpiresInHours
	if hours <= 0 {
		hours = 24
	}
	token, err := portfolio.SignSession(cfg.JWTSecret, accountID, time.Duration(hours)*time.Hour)
	if err != nil {
		return err
	}
	secure := c.Request.TLS != nil || strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")), "https")
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(portfolio.SessionCookieName(), token, hours*3600, "/", "", secure, true)
	return nil
}

func clearPortfolioSession(c *gin.Context) {
	secure := c.Request.TLS != nil || strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")), "https")
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(portfolio.SessionCookieName(), "", -1, "/", "", secure, true)
}

func parseSkillLines(raw string) []entity.PortfolioSkill {
	var skills []entity.PortfolioSkill
	for _, line := range strings.Split(raw, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		runes := []rune(name)
		if len(runes) > 40 {
			name = string(runes[:40])
		}
		skills = append(skills, entity.PortfolioSkill{Name: name})
		if len(skills) == 12 {
			break
		}
	}
	if skills == nil {
		return []entity.PortfolioSkill{}
	}
	return skills
}

func parseProjectSlots(c *gin.Context) []entity.PortfolioProject {
	var projects []entity.PortfolioProject
	for i := 0; i < 4; i++ {
		title := strings.TrimSpace(c.PostForm(fmt.Sprintf("project_title_%d", i)))
		if title == "" {
			continue
		}
		title = trimField(title, 120)
		projects = append(projects, entity.PortfolioProject{
			Title:   title,
			Summary: trimField(c.PostForm(fmt.Sprintf("project_summary_%d", i)), 300),
			URL:     withHTTPScheme(c.PostForm(fmt.Sprintf("project_url_%d", i))),
		})
	}
	if projects == nil {
		return []entity.PortfolioProject{}
	}
	return projects
}

func trimField(raw string, max int) string {
	raw = strings.TrimSpace(raw)
	runes := []rune(raw)
	if len(runes) > max {
		return string(runes[:max])
	}
	return raw
}

func withHTTPScheme(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		return raw
	}
	return "https://" + raw
}

func savePortfolioPhoto(c *gin.Context, site *entity.Portfolio) (string, error) {
	file, header, err := c.Request.FormFile("photo")
	if err != nil {
		return "", errors.New("Pilih file foto.")
	}
	defer file.Close()
	if header.Size > 2<<20 {
		return "", errors.New("Foto maksimal 2 MB.")
	}
	buf := make([]byte, 512)
	n, err := io.ReadFull(file, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", errors.New("Foto gagal dibaca.")
	}
	kind := http.DetectContentType(buf[:n])
	ext := ""
	switch kind {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	case "image/gif":
		ext = ".gif"
	default:
		return "", errors.New("Foto harus JPG, PNG, WebP, atau GIF.")
	}
	dir := filepath.Join("public", "static", "uploads", "portfolios")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", errors.New("Folder foto gagal dibuat.")
	}
	name := fmt.Sprintf("%d-%d%s", site.ID, time.Now().Unix(), ext)
	target := filepath.Join(dir, name)
	out, err := os.Create(target)
	if err != nil {
		return "", errors.New("Foto gagal disimpan.")
	}
	defer out.Close()
	if _, err := out.Write(buf[:n]); err != nil {
		return "", errors.New("Foto gagal disimpan.")
	}
	if _, err := io.Copy(out, file); err != nil {
		return "", errors.New("Foto gagal disimpan.")
	}
	return "/static/uploads/portfolios/" + name, nil
}

func removePortfolioPhoto(urlPath string) {
	const prefix = "/static/uploads/portfolios/"
	if !strings.HasPrefix(urlPath, prefix) {
		return
	}
	name := filepath.Base(urlPath)
	if name == "." || name == string(filepath.Separator) || strings.Contains(name, "..") {
		return
	}
	_ = os.Remove(filepath.Join("public", "static", "uploads", "portfolios", name))
}

func portfolioSiteName(setting entity.SiteSetting) string {
	name := strings.TrimSpace(setting.SiteName)
	if name == "" {
		return "Portfolio"
	}
	return name
}

func portfolioLocation() *time.Location {
	loc, err := time.LoadLocation(clientPublicTimezone)
	if err != nil {
		return time.Local
	}
	return loc
}
