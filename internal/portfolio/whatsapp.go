package portfolio

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

// BuyerWhatsAppLink builds the wa.me link the buyer is sent to instead of Midtrans.
func BuyerWhatsAppLink(adminNumber string, order entity.PortfolioOrder, root string) (string, error) {
	if ValidateWhatsApp(adminNumber) != "" {
		return "", &InputError{Message: "Nomor WhatsApp pembayaran belum diatur di admin."}
	}
	digits := strings.TrimPrefix(NormalizeWhatsApp(adminNumber), "+")
	host := strings.Trim(order.Subdomain+"."+strings.TrimSpace(root), ".")
	text := fmt.Sprintf("Halo, saya ingin membuat portfolio.\n\nNama: %s\nEmail: %s\nWhatsApp: %s\nAlamat: %s\nHarga: %s",
		order.BuyerName, order.Email, order.WhatsApp, host, FormatIDR(order.Amount))
	return "https://wa.me/" + digits + "?text=" + url.QueryEscape(text), nil
}
