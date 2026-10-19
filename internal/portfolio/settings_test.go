package portfolio

import (
	"strings"
	"testing"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

func TestParsePrice(t *testing.T) {
	n, err := ParsePrice("149.000")
	if err != nil || n != 149000 {
		t.Fatalf("%d %v", n, err)
	}
	n, err = ParsePrice("Rp149000")
	if err != nil || n != 149000 {
		t.Fatalf("%d %v", n, err)
	}
	if _, err := ParsePrice("abc"); err == nil {
		t.Fatal("expected invalid price")
	}
}

func TestBuyerWhatsAppLink(t *testing.T) {
	link, err := BuyerWhatsAppLink("0812 0000 0000", entity.PortfolioOrder{
		BuyerName: "Andi",
		Email:     "andi@example.com",
		WhatsApp:  "+628111",
		Subdomain: "andi",
		Amount:    149000,
	}, "portofolioku.id")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(link, "https://wa.me/6281200000000?text=") {
		t.Fatal(link)
	}
	if !strings.Contains(link, "andi.portofolioku.id") && !strings.Contains(link, "andi.portofolioku.id") {
		t.Fatal(link)
	}
	if _, err := BuyerWhatsAppLink("", entity.PortfolioOrder{}, "localhost"); err == nil {
		t.Fatal("expected missing admin number to fail")
	}
}
