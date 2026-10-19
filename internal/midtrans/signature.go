package midtrans

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// VerifySignature checks a Midtrans HTTP notification signature.
// Signature is SHA-512 of order_id + status_code + gross_amount + server_key.
func VerifySignature(orderID, statusCode, grossAmount, serverKey, signatureKey string) bool {
	sig := strings.TrimSpace(signatureKey)
	if sig == "" {
		return false
	}
	orderID = strings.TrimSpace(orderID)
	statusCode = strings.TrimSpace(statusCode)
	serverKey = strings.TrimSpace(serverKey)
	grossAmount = strings.TrimSpace(grossAmount)

	candidates := []string{grossAmount}
	if val, err := strconv.ParseFloat(grossAmount, 64); err == nil {
		formatted2 := fmt.Sprintf("%.2f", val)
		formatted0 := fmt.Sprintf("%.0f", val)
		if formatted2 != grossAmount {
			candidates = append(candidates, formatted2)
		}
		if formatted0 != grossAmount {
			candidates = append(candidates, formatted0)
		}
	}
	for _, gross := range candidates {
		raw := orderID + statusCode + gross + serverKey
		sum := sha512.Sum512([]byte(raw))
		expected := hex.EncodeToString(sum[:])
		if strings.EqualFold(expected, sig) {
			return true
		}
	}
	return false
}
