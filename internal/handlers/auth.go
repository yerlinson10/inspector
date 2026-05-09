package handlers

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"inspector/internal/middleware"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	Username       string
	Password       string
	SessionManager *middleware.SessionManager
	attemptsMu     sync.Mutex
	attempts       map[string]loginAttempt
	maxAttempts    int
	attemptWindow  time.Duration
	blockDuration  time.Duration
}

type loginAttempt struct {
	FailedCount int
	WindowStart time.Time
	BlockedUntil time.Time
}

func NewAuthHandler(username, password string, sessionTTL time.Duration) *AuthHandler {
	return &AuthHandler{
		Username:       username,
		Password:       password,
		SessionManager: middleware.NewSessionManager(sessionTTL),
		attempts:       make(map[string]loginAttempt),
		maxAttempts:    5,
		attemptWindow:  10 * time.Minute,
		blockDuration:  10 * time.Minute,
	}
}

func (h *AuthHandler) ShowLogin(c *gin.Context) {
	next := sanitizeNextPath(c.DefaultQuery("next", "/dashboard"))
	c.HTML(http.StatusOK, "login.html", withViewData(c, gin.H{
		"title": "Login",
		"next":  next,
	}))
}

func (h *AuthHandler) HandleLogin(c *gin.Context) {
	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")
	next := sanitizeNextPath(c.DefaultPostForm("next", "/dashboard"))
	now := time.Now()
	clientKey := strings.TrimSpace(c.ClientIP())

	if retryAfter, blocked := h.isLoginBlocked(clientKey, now); blocked {
		retrySeconds := int(retryAfter.Seconds())
		if retrySeconds < 1 {
			retrySeconds = 1
		}
		c.Header("Retry-After", strconv.Itoa(retrySeconds))
		c.HTML(http.StatusTooManyRequests, "login.html", withViewData(c, gin.H{
			"title": "Login",
			"next":  next,
			"error": "Too many failed attempts. Try again in " + strconv.Itoa(retrySeconds) + "s",
		}))
		return
	}

	if !constantTimeEquals(username, h.Username) || !constantTimeEquals(password, h.Password) {
		h.recordFailedLogin(clientKey, now)
		c.HTML(http.StatusUnauthorized, "login.html", withViewData(c, gin.H{
			"title": "Login",
			"next":  next,
			"error": "Invalid credentials",
		}))
		return
	}
	h.clearFailedLogin(clientKey)

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

func constantTimeEquals(a, b string) bool {
	aHash := sha256.Sum256([]byte(a))
	bHash := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(aHash[:], bHash[:]) == 1
}

func (h *AuthHandler) isLoginBlocked(key string, now time.Time) (time.Duration, bool) {
	if h == nil || key == "" {
		return 0, false
	}
	h.attemptsMu.Lock()
	defer h.attemptsMu.Unlock()
	h.pruneLoginAttemptsLocked(now)

	attempt, ok := h.attempts[key]
	if !ok {
		return 0, false
	}
	if !attempt.BlockedUntil.IsZero() && now.Before(attempt.BlockedUntil) {
		return attempt.BlockedUntil.Sub(now), true
	}
	if !attempt.BlockedUntil.IsZero() && !now.Before(attempt.BlockedUntil) {
		delete(h.attempts, key)
	}
	return 0, false
}

func (h *AuthHandler) recordFailedLogin(key string, now time.Time) {
	if h == nil || key == "" {
		return
	}
	h.attemptsMu.Lock()
	defer h.attemptsMu.Unlock()

	attempt := h.attempts[key]
	if attempt.WindowStart.IsZero() || now.Sub(attempt.WindowStart) > h.attemptWindow {
		attempt = loginAttempt{WindowStart: now}
	}
	attempt.FailedCount++
	if h.maxAttempts > 0 && attempt.FailedCount >= h.maxAttempts {
		attempt.BlockedUntil = now.Add(h.blockDuration)
		attempt.FailedCount = 0
		attempt.WindowStart = now
	}
	h.attempts[key] = attempt
	h.pruneLoginAttemptsLocked(now)
}

func (h *AuthHandler) clearFailedLogin(key string) {
	if h == nil || key == "" {
		return
	}
	h.attemptsMu.Lock()
	delete(h.attempts, key)
	h.attemptsMu.Unlock()
}

func (h *AuthHandler) pruneLoginAttemptsLocked(now time.Time) {
	if h == nil {
		return
	}
	for key, attempt := range h.attempts {
		if !attempt.BlockedUntil.IsZero() {
			if now.After(attempt.BlockedUntil.Add(h.attemptWindow)) {
				delete(h.attempts, key)
			}
			continue
		}
		if !attempt.WindowStart.IsZero() && now.Sub(attempt.WindowStart) > h.attemptWindow {
			delete(h.attempts, key)
		}
	}
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
