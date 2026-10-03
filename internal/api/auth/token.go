package auth

import (
	"errors"
	"time"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

type TokenManager struct {
	key           []byte
	tokenLifetime int
}

func NewTokenManager(secret string, tokenLifetime int) (*TokenManager, error) {
	if len(secret) < 32 {
		return nil, errors.New("invalid jwt secret, need secret atleast 32 characters long")
	}
	if tokenLifetime <= 0 {
		return nil, errors.New("invalid jwt token life time duration, need non-zero and non-negative value")
	}
	return &TokenManager{[]byte(secret), tokenLifetime}, nil
}

type CustomClaims struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	TenantID string `json:"tenantid"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func (m *TokenManager) IssueToken(user *domain.User) (string, error) {
	now := time.Now()
	expirationTime := now.Add(time.Hour * time.Duration(m.tokenLifetime))

	claims := &CustomClaims{
		ID:       user.ID,
		Email:    user.Email,
		TenantID: user.TenantID,
		Role:     string(user.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "luma",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(m.key)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (m *TokenManager) ParseToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(t *jwt.Token) (any, error) {
		return m.key, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer("luma"))
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid or expired token")
}
