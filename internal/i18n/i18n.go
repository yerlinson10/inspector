package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

const (
	DefaultLanguage      = "en"
	ContextTranslatorKey = "t"
	ContextLanguageKey   = "lang"
)

//go:embed locales/*.json
var localeFS embed.FS

var (
	bundle   *goi18n.Bundle
	initOnce sync.Once
	initErr  error
)

func Init(defaultLang string) error {
	initOnce.Do(func() {
		bundle = goi18n.NewBundle(language.Make(normalizeLanguage(defaultLang)))
		bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

		for _, lang := range []string{"es", "en"} {
			path := fmt.Sprintf("locales/%s.json", lang)
			if _, err := bundle.LoadMessageFileFS(localeFS, path); err != nil {
				initErr = fmt.Errorf("failed to load locale file %s: %w", path, err)
				return
			}
		}
	})

	return initErr
}

func Localizer(lang string) *goi18n.Localizer {
	if bundle == nil {
		_ = Init(DefaultLanguage)
	}
	return goi18n.NewLocalizer(bundle, normalizeLanguage(lang))
}

func normalizeLanguage(raw string) string {
	lang := strings.ToLower(strings.TrimSpace(raw))
	if lang == "" {
		return DefaultLanguage
	}
	if strings.HasPrefix(lang, "en") {
		return "en"
	}
	if strings.HasPrefix(lang, "es") {
		return "es"
	}
	return DefaultLanguage
}
