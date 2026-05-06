package handlers

import (
	"net/http"
	"strings"

	"inspector/internal/middleware"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	Username     string
	Password     string
	SessionValue string
}

func NewAuthHandler(username, password string) *AuthHandler {
	return &AuthHandler{
		Username:     username,
		Password:     password,
		SessionValue: middleware.BuildSessionValue(username, password),
	}
}

func (h *AuthHandler) ShowLogin(c *gin.Context) {
	next := c.DefaultQuery("next", "/dashboard")
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "Login",
		"next":  next,
	})
}

func (h *AuthHandler) HandleLogin(c *gin.Context) {
	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")
	next := c.DefaultPostForm("next", "/dashboard")

	if !strings.HasPrefix(next, "/") {
		next = "/dashboard"
	}

	if username != h.Username || password != h.Password {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"title": "Login",
			"next":  next,
			"error": "Credenciales invalidas",
		})
		return
	}

	secureCookie := isSecureRequest(c)
	if secureCookie {
		c.SetSameSite(http.SameSiteNoneMode)
	} else {
		c.SetSameSite(http.SameSiteLaxMode)
	}
	c.SetCookie(middleware.SessionCookieName, h.SessionValue, 3600*12, "/", "", secureCookie, true)
	c.Redirect(http.StatusSeeOther, next)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	secureCookie := isSecureRequest(c)
	if secureCookie {
		c.SetSameSite(http.SameSiteNoneMode)
	} else {
		c.SetSameSite(http.SameSiteLaxMode)
	}
	c.SetCookie(middleware.SessionCookieName, "", -1, "/", "", secureCookie, true)
	c.Redirect(http.StatusSeeOther, "/login")
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
