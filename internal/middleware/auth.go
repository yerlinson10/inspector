package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"

	"github.com/gin-gonic/gin"
)

const SessionCookieName = "inspector_session"

func BuildSessionValue(username, password string) string {
	sum := sha256.Sum256([]byte(username + ":" + password))
	return hex.EncodeToString(sum[:])
}

func SessionAuth(sessionValue string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(SessionCookieName)
		if err != nil || cookie != sessionValue {
			next := url.QueryEscape(c.Request.URL.RequestURI())
			c.Redirect(302, "/login?next="+next)
			c.Abort()
			return
		}
		c.Next()
	}
}
