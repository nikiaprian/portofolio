package midtrans

import (
	"crypto/sha512"
	"encoding/hex"
	"testing"
)

func TestVerifySignature(t *testing.T) {
	orderID := "PF-abc"
	statusCode := "200"
	gross := "149000.00"
	serverKey := "SB-Mid-server"
	raw := orderID + statusCode + gross + serverKey
	sum := sha512.Sum512([]byte(raw))
	sig := hex.EncodeToString(sum[:])

	if !VerifySignature(orderID, statusCode, gross, serverKey, sig) {
		t.Fatal("expected exact gross amount to verify")
	}
	if !VerifySignature(orderID, statusCode, "149000", serverKey, sig) {
		t.Fatal("expected integer gross amount to match .00 signature")
	}
	if VerifySignature(orderID, statusCode, gross, serverKey, "nope") {
		t.Fatal("expected bad signature to fail")
	}
	if VerifySignature(orderID, statusCode, gross, serverKey, "") {
		t.Fatal("expected empty signature to fail")
	}
}
