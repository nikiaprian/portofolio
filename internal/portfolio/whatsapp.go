package portfolio

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

// BuyerWhatsAppLink builds the wa.me link the buyer opens after the purchase form.
func BuyerWhatsAppLink(adminNumber string, order entity.PortfolioOrder, root, setupURL string) (string, error) {
	if ValidateWhatsApp(adminNumber) != "" {
		return "", &InputError{Message: "Nomor WhatsApp pembayaran belum diatur di admin."}
	}
	digits := strings.TrimPrefix(NormalizeWhatsApp(adminNumber), "+")
	host := strings.Trim(order.Subdomain+"."+strings.TrimSpace(root), ".")
	templateName := strings.TrimSpace(order.Template)
	if item, ok := TemplateBySlug(templateName); ok {
		templateName = item.Name
	}
	if templateName == "" {
		templateName = "Portfolio"
	}
	text := fmt.Sprintf("Halo, saya ingin membeli portfolio.\n\nTemplate: %s\nNama: %s\nEmail: %s\nWhatsApp: %s\nAlamat: %s\nHarga: %s",
		templateName, order.BuyerName, order.Email, order.WhatsApp, host, FormatIDR(order.Amount))
	if setup := strings.TrimSpace(setupURL); setup != "" {
		text += "\n\nSetelah pembayaran dikonfirmasi dan portfolio diaktifkan, saya atur password di:\n" + setup
	}
	return "https://wa.me/" + digits + "?text=" + url.QueryEscape(text), nil
}
