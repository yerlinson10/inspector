package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const SessionCookieName = "inspector_session"

type SessionManager struct {
	mu       sync.RWMutex
	ttl      time.Duration
	sessions map[string]time.Time
}

func NewSessionManager(ttl time.Duration) *SessionManager {
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	return &SessionManager{
		ttl:      ttl,
		sessions: make(map[string]time.Time),
	}
}

func (m *SessionManager) CreateSession() string {
	token := randomSessionToken()
	now := time.Now()
	m.mu.Lock()
	m.cleanupExpiredLocked(now)
	m.sessions[token] = now.Add(m.ttl)
	m.mu.Unlock()
	return token
}

func (m *SessionManager) ValidateSession(token string) bool {
	if token == "" {
		return false
	}

	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	expiresAt, exists := m.sessions[token]
	if !exists {
		return false
	}
	if now.After(expiresAt) {
		delete(m.sessions, token)
		return false
	}

	// Sliding expiration keeps active sessions valid while expiring idle ones.
	m.sessions[token] = now.Add(m.ttl)
	return true
}

func (m *SessionManager) DeleteSession(token string) {
	if token == "" {
		return
	}
	m.mu.Lock()
	delete(m.sessions, token)
	m.mu.Unlock()
}

func (m *SessionManager) cleanupExpiredLocked(now time.Time) {
	for token, expiresAt := range m.sessions {
		if now.After(expiresAt) {
			delete(m.sessions, token)
		}
	}
}

func (m *SessionManager) TTL() time.Duration {
	if m == nil {
		return 0
	}
	return m.ttl
}

func randomSessionToken() string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return uuid.NewString()
	}
	return hex.EncodeToString(raw)
}

func SessionAuth(validateSession func(string) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(SessionCookieName)
		if err != nil || validateSession == nil || !validateSession(cookie) {
			next := url.QueryEscape(c.Request.URL.RequestURI())
			c.Redirect(302, "/login?next="+next)
			c.Abort()
			return
		}
		c.Next()
	}
}
