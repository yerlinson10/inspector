package i18n

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// GetLangFromRequest obtiene el idioma preferido desde query param, cookie o Accept-Language.
func GetLangFromRequest(c *gin.Context) string {
	if raw := strings.TrimSpace(c.Query("lang")); raw != "" {
		return normalizeLanguage(raw)
	}

	if lang, err := c.Cookie("lang"); err == nil && lang != "" {
		return normalizeLanguage(lang)
	}

	al := c.GetHeader("Accept-Language")
	if len(al) >= 2 {
		return normalizeLanguage(al[:2])
	}

	return DefaultLanguage
}
