package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

const (
	AuthHeader    = "Authorization"
	AuthScheme    = "Bearer"
	APIKeyPurpose = "api-key"
)

type AuthMiddleware struct {
	apiKeyHash []byte
}

func NewAuthMiddleware(identityDerivedKey []byte) *AuthMiddleware {
	h := sha256.Sum256(identityDerivedKey)
	return &AuthMiddleware{apiKeyHash: h[:]}
}

func DeriveAPIKey(identityDerivedKey []byte) string {
	return hex.EncodeToString(identityDerivedKey)
}

func (a *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get(AuthHeader)
		if authHeader == "" {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != AuthScheme {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"invalid authorization scheme, use Bearer"}`, http.StatusUnauthorized)
			return
		}

		providedKey, err := hex.DecodeString(parts[1])
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"invalid api key format"}`, http.StatusUnauthorized)
			return
		}

		providedHash := sha256.Sum256(providedKey)
		if !hmac.Equal(providedHash[:], a.apiKeyHash) {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"invalid api key"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
