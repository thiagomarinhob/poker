package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	accessTokenDuration  = 15 * time.Minute
	refreshTokenDuration = 7 * 24 * time.Hour
)

type claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func generateAccessToken(userID uuid.UUID, role, secret string) (string, error) {
	now := time.Now()
	c := claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenDuration)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(secret))
}

// UserFromAccessToken valida o JWT (mesmo do header Bearer) e retorna subject e role.
func UserFromAccessToken(tokenStr, secret string) (string, string, error) {
	c, err := validateAccessToken(tokenStr, secret)
	if err != nil {
		return "", "", err
	}
	if c.Subject == "" {
		return "", "", fmt.Errorf("empty subject")
	}
	return c.Subject, c.Role, nil
}

func validateAccessToken(tokenStr, secret string) (*claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := token.Claims.(*claims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}
	return c, nil
}

func newRefreshToken() string {
	return uuid.New().String()
}

func refreshKey(token string) string {
	return "refresh_token:" + token
}
