// Package auth owns credential hashing and JWT issuance/parsing. It is the
// only package that knows the JWT secret or the bcrypt cost, keeping security
// concerns in one auditable place.
package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Auth carries the signing secret and token lifetime. Construct once at startup.
type Auth struct {
	secret []byte
	expiry time.Duration
	clock  func() time.Time
}

// New returns an Auth configured with the given secret and expiry.
// An empty secret is rejected to prevent accidental insecure deployments.
func New(secret string, expiry time.Duration) (*Auth, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("auth: JWT secret must be set")
	}
	return &Auth{secret: []byte(secret), expiry: expiry, clock: time.Now}, nil
}

// Claims is the JWT payload. UserID is the subject; Email is included for logs.
type Claims struct {
	UserID string `json:"uid"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// Issue signs a token for the given user valid for the configured expiry.
func (a *Auth) Issue(userID, email string) (string, error) {
	now := a.clock()
	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.expiry)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(a.secret)
	if err != nil {
		return "", fmt.Errorf("auth: sign: %w", err)
	}
	return signed, nil
}

// Parse validates the token signature and expiry, returning the claims on success.
func (a *Auth) Parse(token string) (*Claims, error) {
	var claims Claims
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	_, err := parser.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method %v", t.Method.Alg())
		}
		return a.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("auth: parse: %w", err)
	}
	return &claims, nil
}

// HashPassword returns a bcrypt hash of the plaintext password.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("auth: bcrypt: %w", err)
	}
	return string(b), nil
}

// CheckPassword reports whether plaintext matches the stored hash.
func CheckPassword(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
