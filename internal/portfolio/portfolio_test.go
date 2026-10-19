package portfolio

import (
	"strings"
	"testing"
	"time"
)

func TestClassifyHost(t *testing.T) {
	kind, sub := ClassifyHost("localhost:8080", "localhost")
	if kind != HostApex || sub != "" {
		t.Fatalf("apex localhost: %s %s", kind, sub)
	}
	kind, sub = ClassifyHost("budi.localhost:8080", "localhost")
	if kind != HostTenant || sub != "budi" {
		t.Fatalf("tenant: %s %s", kind, sub)
	}
	kind, sub = ClassifyHost("www.example.com", "example.com")
	if kind != HostApex {
		t.Fatalf("www should be apex, got %s", kind)
	}
	kind, sub = ClassifyHost("admin.example.com", "example.com")
	if kind != HostReserved || sub != "admin" {
		t.Fatalf("reserved: %s %s", kind, sub)
	}
	kind, _ = ClassifyHost("127.0.0.1:8080", "localhost")
	if kind != HostApex {
		t.Fatalf("ip should be apex, got %s", kind)
	}
}

func TestValidateSubdomain(t *testing.T) {
	if msg := ValidateSubdomain("budi"); msg != "" {
		t.Fatal(msg)
	}
	if msg := ValidateSubdomain("Bu"); msg == "" {
		t.Fatal("expected short name to fail")
	}
	if msg := ValidateSubdomain("-budi"); msg == "" {
		t.Fatal("expected leading hyphen to fail")
	}
	if msg := ValidateSubdomain("kelola"); msg == "" {
		t.Fatal("expected reserved name to fail")
	}
}

func TestSubdomainHeld(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	if !SubdomainHeld("active", now.Add(-48*time.Hour), now) {
		t.Fatal("active should hold")
	}
	if !SubdomainHeld("pending", now.Add(-2*time.Hour), now) {
		t.Fatal("fresh pending should hold")
	}
	if SubdomainHeld("pending", now.Add(-25*time.Hour), now) {
		t.Fatal("stale pending should not hold")
	}
	if !SubdomainHeld("suspended", now, now) {
		t.Fatal("suspended should hold")
	}
}

func TestNormalizeWhatsApp(t *testing.T) {
	if got := NormalizeWhatsApp("0812-3456-7890"); got != "+6281234567890" {
		t.Fatal(got)
	}
	if msg := ValidateWhatsApp("123"); msg == "" {
		t.Fatal("expected short number to fail")
	}
}

func TestFormatIDR(t *testing.T) {
	if got := FormatIDR(149000); got != "Rp149.000" {
		t.Fatal(got)
	}
	if strings.Contains(FormatIDR(1500), ",") {
		t.Fatal(FormatIDR(1500))
	}
}
