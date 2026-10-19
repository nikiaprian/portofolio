package portfolio

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const sessionCookie = "portfolio_token"

func SessionCookieName() string { return sessionCookie }

type sessionClaims struct {
	AccountID uint `json:"account_id"`
	jwt.RegisteredClaims
}

func sessionSecret(jwtSecret string) string {
	return jwtSecret + ":portfolio"
}

func SignSession(jwtSecret string, accountID uint, ttl time.Duration) (string, error) {
	if accountID == 0 {
		return "", errors.New("missing account")
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	now := time.Now()
	claims := sessionClaims{
		AccountID: accountID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "portfolio",
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(sessionSecret(jwtSecret)))
}

func ParseSession(jwtSecret, tokenString string) (uint, error) {
	token, err := jwt.ParseWithClaims(tokenString, &sessionClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(sessionSecret(jwtSecret)), nil
	})
	if err != nil {
		return 0, err
	}
	claims, ok := token.Claims.(*sessionClaims)
	if !ok || !token.Valid || claims.Subject != "portfolio" || claims.AccountID == 0 {
		return 0, errors.New("invalid token")
	}
	return claims.AccountID, nil
}
