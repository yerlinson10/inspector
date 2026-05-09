package handlers

import (
	"inspector/internal/i18n"

	"github.com/gin-gonic/gin"
)

func withViewData(c *gin.Context, data gin.H) gin.H {
	if data == nil {
		data = gin.H{}
	}

	if _, exists := data["t"]; !exists {
		if t, ok := c.Get(i18n.ContextTranslatorKey); ok {
			if tFunc, ok := t.(func(string) string); ok {
				data["t"] = tFunc
			}
		}
	}
	if _, exists := data["t"]; !exists {
		data["t"] = func(id string) string { return id }
	}

	if _, exists := data["lang"]; !exists {
		if lang, ok := c.Get(i18n.ContextLanguageKey); ok {
			if langStr, ok := lang.(string); ok && langStr != "" {
				data["lang"] = langStr
			}
		}
	}
	if _, exists := data["lang"]; !exists {
		data["lang"] = i18n.DefaultLanguage
	}

	if _, exists := data["langSwitchURLEN"]; !exists {
		q := c.Request.URL.Query()
		q.Set("lang", "en")
		data["langSwitchURLEN"] = c.Request.URL.Path + "?" + q.Encode()
	}
	if _, exists := data["langSwitchURLES"]; !exists {
		q := c.Request.URL.Query()
		q.Set("lang", "es")
		data["langSwitchURLES"] = c.Request.URL.Path + "?" + q.Encode()
	}

	if _, exists := data["csrfToken"]; !exists {
		if token, ok := c.Get("csrfToken"); ok {
			if tokenStr, ok := token.(string); ok {
				data["csrfToken"] = tokenStr
			}
		}
	}
	return data
}
