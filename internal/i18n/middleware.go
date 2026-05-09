package i18n

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

// TFunc devuelve una función de traducción para usar en plantillas
func TFunc(localizer *goi18n.Localizer) func(string) string {
	return func(id string) string {
		msg, err := localizer.Localize(&goi18n.LocalizeConfig{MessageID: id})
		if err != nil {
			return id // fallback
		}
		return msg
	}
}

// Middleware para inyectar la función t en el contexto de las plantillas
func I18nMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := GetLangFromRequest(c)

		if raw := strings.TrimSpace(c.Query("lang")); raw != "" {
			normalized := normalizeLanguage(raw)
			secureCookie := c.Request.TLS != nil || strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")), "https")
			c.SetSameSite(http.SameSiteLaxMode)
			c.SetCookie("lang", normalized, 365*24*3600, "/", "", secureCookie, false)
			lang = normalized
		}

		localizer := Localizer(lang)
		t := TFunc(localizer)
		c.Set(ContextTranslatorKey, t)
		c.Set(ContextLanguageKey, lang)
		c.Next()
	}
}
