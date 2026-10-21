package routes

import (
	"bytes"
	"html/template"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Bebii-Digital/bebii-blog/config"
)

func TestPortfolioTemplatesParse(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	layout := filepath.Join(root, "public", "templates", "layouts", "portfolio", "app.html")
	pages := []string{
		"landing.html", "checkout.html", "pay.html", "done.html",
		"public.html", "missing.html", "login.html", "password.html", "manage.html",
		"blog.html", "article.html", "info.html", "testimoni.html", "error.html", "rss.html",
		"products.html", "product.html", "themes.html",
	}
	for _, name := range pages {
		path := filepath.Join(root, "public", "templates", "pages", "portfolio", name)
		if _, err := template.ParseFiles(layout, path); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := template.ParseFiles(
		filepath.Join(root, "public", "templates", "layouts", "admin", "app.html"),
		filepath.Join(root, "public", "templates", "layouts", "admin", "navbar.html"),
		filepath.Join(root, "public", "templates", "layouts", "admin", "sidebar.html"),
		filepath.Join(root, "public", "templates", "pages", "admin", "portfolio", "index.html"),
	); err != nil {
		t.Fatal(err)
	}
}

func TestPortfolioBuyerRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerPortfolioBuyerRoutes(r, nil, config.Config{PortfolioRootDomain: "localhost", PortfolioPrice: 149000})
	found := map[string]bool{}
	for _, route := range r.Routes() {
		found[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"GET /produk",
		"GET /produk/:slug",
		"GET /testimoni",
		"GET /beli",
		"POST /beli",
		"GET /beli/bayar/:ref",
		"GET /beli/selesai/:ref",
		"GET /kelola",
		"GET /kelola/masuk",
		"POST /kelola/masuk",
		"POST /kelola/keluar",
		"GET /kelola/atur-password/:token",
		"POST /kelola/atur-password/:token",
		"POST /kelola",
		"POST /kelola/foto",
	} {
		if !found[want] {
			t.Fatalf("missing route %s", want)
		}
	}
}

func TestPortfolioPagesRenderCopy(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	layout := filepath.Join(root, "public", "templates", "layouts", "portfolio", "app.html")
	pages := filepath.Join(root, "public", "templates", "pages", "portfolio")
	landing := filepath.Join(pages, "landing.html")
	themes := filepath.Join(pages, "themes.html")
	tmpl, err := template.ParseFiles(layout, themes, landing)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "pf-app", map[string]any{
		"Title":       "Portfolio online",
		"Description": "Beli portfolio online",
		"SiteName":    "Portfolio",
		"Root":        "localhost",
		"PriceLabel":  "Rp149.000",
		"DemoURL":     "http://contoh.localhost:8080/",
		"Articles": []map[string]string{{
			"Title": "Cara menata portfolio",
			"Href":  "/cara-menata-portfolio",
			"Date":  "1 Jan 2026",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	body := buf.String()
	for _, needle := range []string{"langsung kelihatan", "Rp149.000", "Cara menata portfolio", "/posts", "contoh.localhost"} {
		if !strings.Contains(body, needle) {
			t.Fatalf("landing missing %q", needle)
		}
	}
	publicTmpl, err := template.ParseFiles(layout, themes, filepath.Join(pages, "public.html"))
	if err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	base := map[string]any{
		"DisplayName": "Contoh Kreator",
		"Headline":    "Desainer produk",
		"Bio":         "Bio contoh.",
		"Initials":    "CK",
		"Email":       "halo@contoh.test",
		"WhatsAppURL": "https://wa.me/6281234567890",
		"Skills":      []map[string]string{{"Name": "Riset"}},
		"Projects":    []map[string]string{{"Title": "Situs jurnal", "Summary": "Alur baca", "URL": "https://example.com"}},
		"Website":     "https://example.com",
	}
	if err := publicTmpl.ExecuteTemplate(&buf, "pf-app", base); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "pf-theme-klasik") || !strings.Contains(buf.String(), "Contoh Kreator") || !strings.Contains(buf.String(), "Situs jurnal") {
		t.Fatal("public portfolio missing content")
	}
	for _, slug := range []string{"studio", "editorial", "galeri", "lamaran", "kreator"} {
		buf.Reset()
		data := map[string]any{}
		for key, value := range base {
			data[key] = value
		}
		data["Template"] = slug
		if err := publicTmpl.ExecuteTemplate(&buf, "pf-app", data); err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
		if !strings.Contains(buf.String(), "pf-theme-"+slug) {
			t.Fatalf("%s layout missing", slug)
		}
	}
	productTmpl, err := template.ParseFiles(layout, themes, filepath.Join(pages, "product.html"))
	if err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	preview := map[string]any{}
	for key, value := range base {
		preview[key] = value
	}
	preview["Nav"] = "none"
	preview["Template"] = "studio"
	preview["ThemeClass"] = "pf-body-studio"
	preview["Title"] = "Studio"
	if err := productTmpl.ExecuteTemplate(&buf, "pf-app", preview); err != nil {
		t.Fatal(err)
	}
	previewHTML := buf.String()
	if strings.Contains(previewHTML, `class="pf-top"`) || strings.Contains(previewHTML, "pf-sitefoot") || !strings.Contains(previewHTML, "pf-theme-studio") {
		t.Fatal("product preview should be the bare portfolio")
	}
	listTmpl, err := template.ParseFiles(layout, themes, filepath.Join(pages, "products.html"))
	if err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := listTmpl.ExecuteTemplate(&buf, "pf-app", map[string]any{
		"Title": "Produk",
		"Products": []map[string]string{{
			"Slug": "klasik", "Name": "Klasik", "Summary": "Tenang",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	listHTML := buf.String()
	if !strings.Contains(listHTML, `href="/produk/klasik" target="_blank"`) {
		t.Fatal("product card should open a new tab")
	}
}
