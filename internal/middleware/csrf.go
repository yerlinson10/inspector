package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const CSRFCookieName = "inspector_csrf"

func CSRFProtection() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(CSRFCookieName)
		if err != nil || len(token) < 32 {
			token = randomCSRFToken()
			secureCookie := isSecureCSRFRequest(c)
			c.SetSameSite(http.SameSiteLaxMode)
			c.SetCookie(CSRFCookieName, token, 3600*12, "/", "", secureCookie, false)
		}

		c.Set("csrfToken", token)

		if isMutatingMethod(c.Request.Method) {
			receivedToken := strings.TrimSpace(c.GetHeader("X-CSRF-Token"))
			if receivedToken == "" {
				receivedToken = strings.TrimSpace(c.PostForm("csrf_token"))
			}

			if receivedToken == "" || subtle.ConstantTimeCompare([]byte(receivedToken), []byte(token)) != 1 {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid csrf token"})
				return
			}
		}

		c.Next()
	}
}

func randomCSRFToken() string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return uuid.NewString()
	}
	return hex.EncodeToString(raw)
}

func isMutatingMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isSecureCSRFRequest(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	if strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		return true
	}
	if strings.EqualFold(c.GetHeader("X-Forwarded-Ssl"), "on") {
		return true
	}
	return false
}
