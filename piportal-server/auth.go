package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const userContextKey contextKey = "user"

// HashPassword hashes a password with bcrypt cost 10
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	return string(hash), err
}

// CheckPassword verifies a password against a bcrypt hash
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GenerateJWT creates a signed JWT with the user ID as subject
func GenerateJWT(userID, secret string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateJWT parses and validates a JWT, returning the user ID
func ValidateJWT(tokenStr, secret string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return "", err
	}
	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return "", jwt.ErrTokenInvalidClaims
	}
	return claims.Subject, nil
}

// SetAuthCookie sets the JWT as an httpOnly cookie
func SetAuthCookie(w http.ResponseWriter, token string, devMode bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   !devMode,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours
	})
}

// ClearAuthCookie removes the auth cookie
func ClearAuthCookie(w http.ResponseWriter, devMode bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   !devMode,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// AuthMiddleware extracts the user from JWT (Bearer header or cookie) and adds to context.
// Returns the user or nil if not authenticated.
func (h *Handler) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var tokenStr string

		// Check Authorization header first
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			tokenStr = strings.TrimPrefix(auth, "Bearer ")
		}

		// Fall back to cookie
		if tokenStr == "" {
			if cookie, err := r.Cookie("token"); err == nil {
				tokenStr = cookie.Value
			}
		}

		if tokenStr == "" {
			jsonError(w, "Authentication required", http.StatusUnauthorized)
			return
		}

		userID, err := ValidateJWT(tokenStr, h.config.JWTSecret)
		if err != nil {
			jsonError(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		user, err := h.store.GetUserByID(userID)
		if err != nil || user == nil {
			jsonError(w, "User not found", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next(w, r.WithContext(ctx))
	}
}

// UserFromContext extracts the user from the request context
func UserFromContext(r *http.Request) *User {
	user, _ := r.Context().Value(userContextKey).(*User)
	return user
}
