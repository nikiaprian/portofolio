package middleware

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTManager struct
type JWTManager struct {
	secret  string
	expires time.Duration
}

// function to create a new JWTManager
func NewJWTManager(secret string, expiresInHours int) *JWTManager {
	if expiresInHours <= 0 {
		expiresInHours = 24
	}
	return &JWTManager{
		secret:  secret,
		expires: time.Duration(expiresInHours) * time.Hour,
	}
}

// Claims struct
type Claims struct {
	UserID uint `json:"user_id"`
	jwt.RegisteredClaims
}

// function to generate a new JWT token
func (m *JWTManager) Generate(userID uint) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "",
			ExpiresAt: jwt.NewNumericDate(now.Add(m.expires)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.secret))
}

// function to verify a JWT token
func (m *JWTManager) Verify(tokenString string) (uint, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(m.secret), nil
	})
	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return 0, errors.New("invalid token")
	}

	return claims.UserID, nil
}
