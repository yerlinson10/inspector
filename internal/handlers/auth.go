package handlers

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"inspector/internal/middleware"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	Username       string
	Password       string
	SessionManager *middleware.SessionManager
}

func NewAuthHandler(username, password string, sessionTTL time.Duration) *AuthHandler {
	return &AuthHandler{
		Username:       username,
		Password:       password,
		SessionManager: middleware.NewSessionManager(sessionTTL),
	}
}

func (h *AuthHandler) ShowLogin(c *gin.Context) {
	next := sanitizeNextPath(c.DefaultQuery("next", "/dashboard"))
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "Login",
		"next":  next,
	})
}

func (h *AuthHandler) HandleLogin(c *gin.Context) {
	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")
	next := sanitizeNextPath(c.DefaultPostForm("next", "/dashboard"))

	if username != h.Username || password != h.Password {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"title": "Login",
			"next":  next,
			"error": "Credenciales invalidas",
		})
		return
	}

	if h.SessionManager == nil {
		h.SessionManager = middleware.NewSessionManager(12 * time.Hour)
	}

	sessionToken := h.SessionManager.CreateSession()
	maxAge := int(h.SessionManager.TTL().Seconds())
	if maxAge <= 0 {
		maxAge = 3600 * 12
	}

	secureCookie := isSecureRequest(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(middleware.SessionCookieName, sessionToken, maxAge, "/", "", secureCookie, true)
	c.Redirect(http.StatusSeeOther, next)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if sessionToken, err := c.Cookie(middleware.SessionCookieName); err == nil && h.SessionManager != nil {
		h.SessionManager.DeleteSession(sessionToken)
	}

	secureCookie := isSecureRequest(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(middleware.SessionCookieName, "", -1, "/", "", secureCookie, true)
	c.Redirect(http.StatusSeeOther, "/login")
}

func (h *AuthHandler) ValidateSession(token string) bool {
	if h == nil || h.SessionManager == nil {
		return false
	}
	return h.SessionManager.ValidateSession(token)
}

func sanitizeNextPath(next string) string {
	next = strings.TrimSpace(next)
	if next == "" {
		return "/dashboard"
	}
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.Contains(next, "\\") {
		return "/dashboard"
	}

	parsed, err := url.Parse(next)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return "/dashboard"
	}

	return parsed.RequestURI()
}

func isSecureRequest(c *gin.Context) bool {
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
