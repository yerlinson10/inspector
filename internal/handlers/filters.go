package handlers

import (
	"net/url"
	"strings"
	"time"
)

func parseTimeFilter(raw string, endOfDay bool) (time.Time, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, false
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			if layout == "2006-01-02" && endOfDay {
				parsed = parsed.Add(24*time.Hour - time.Nanosecond)
			}
			return parsed, true
		}
	}

	return time.Time{}, false
}

func buildFilterQuery(filters map[string]string) string {
	values := url.Values{}
	for key, value := range filters {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		values.Set(key, trimmed)
	}
	return values.Encode()
}
