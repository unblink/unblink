package relay

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWT claims structure
type JWTClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// JWTManager handles JWT token generation and validation
type JWTManager struct {
	secretKey string
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(secretKey string) *JWTManager {
	return &JWTManager{
		secretKey: secretKey,
	}
}

// GenerateToken generates a JWT token for a user
func (m *JWTManager) GenerateToken(userID string, email string) (string, error) {
	if m.secretKey == "" {
		return "", errors.New("JWT secret key not configured")
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	claims := JWTClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	fmt.Printf("[JWT] Generating token - UserID: %s, Email: %s, ExpiresAt: %v\n",
		userID, email, expiresAt)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(m.secretKey))
	if err == nil {
		fmt.Printf("[JWT] Token generated successfully (first 20 chars): %s...\n", tokenString[:min(20, len(tokenString))])
	}
	return tokenString, err
}

// ValidateToken validates a JWT token and returns the claims
func (m *JWTManager) ValidateToken(tokenString string) (*JWTClaims, error) {
	if m.secretKey == "" {
		return nil, errors.New("JWT secret key not configured")
	}

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.secretKey), nil
	})

	if err != nil {
		fmt.Printf("[JWT] Token parsing failed: %v\n", err)
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		fmt.Printf("[JWT] Invalid token claims or invalid token\n")
		return nil, errors.New("invalid token claims")
	}

	// Check expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		fmt.Printf("[JWT] Token expired - ExpiresAt: %v, Now: %v\n", claims.ExpiresAt, time.Now())
		return nil, errors.New("token expired")
	}

	return claims, nil
}

// ExtractUserID extracts user ID from token (convenience method)
func (m *JWTManager) ExtractUserID(tokenString string) (string, error) {
	claims, err := m.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}
