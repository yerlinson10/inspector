package handlers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"inspector/internal/models"
)

const redactedValue = "[REDACTED]"

var redactionPairRegex = regexp.MustCompile(`(?i)((?:token|password|secret|api[_-]?key|authorization)\s*[:=]\s*)([^\s,;]+)`)

func redactRequestLog(input models.RequestLog) models.RequestLog {
	enabled, headerLookup, fieldLookup := redactionSettingsSnapshot()
	if !enabled {
		return input
	}

	input.Headers = redactHeadersSerialized(input.Headers, headerLookup)
	input.QueryParams = redactQueryParamsSerialized(input.QueryParams, fieldLookup)
	input.Body = redactBodySerialized(input.Body, fieldLookup)
	return input
}

func redactSentRequest(input models.SentRequest) models.SentRequest {
	enabled, headerLookup, fieldLookup := redactionSettingsSnapshot()
	if !enabled {
		return input
	}

	input.URL = redactURLSensitiveQuery(input.URL, fieldLookup)
	input.Headers = redactHeadersSerialized(input.Headers, headerLookup)
	input.Body = redactBodySerialized(input.Body, fieldLookup)
	input.ResponseHeaders = redactHeadersSerialized(input.ResponseHeaders, headerLookup)
	input.ResponseBody = redactBodySerialized(input.ResponseBody, fieldLookup)
	input.Error = redactBodySerialized(input.Error, fieldLookup)
	return input
}

func redactHeadersSerialized(raw string, headerLookup map[string]struct{}) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}

	var multi map[string][]string
	if err := json.Unmarshal([]byte(trimmed), &multi); err == nil {
		for key := range multi {
			if isSensitiveHeaderKey(key, headerLookup) {
				multi[key] = []string{redactedValue}
			}
		}
		encoded, err := json.Marshal(multi)
		if err == nil {
			return string(encoded)
		}
	}

	var single map[string]string
	if err := json.Unmarshal([]byte(trimmed), &single); err == nil {
		for key := range single {
			if isSensitiveHeaderKey(key, headerLookup) {
				single[key] = redactedValue
			}
		}
		encoded, err := json.Marshal(single)
		if err == nil {
			return string(encoded)
		}
	}

	lines := strings.Split(raw, "\n")
	changed := false
	for i, line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			continue
		}
		if isSensitiveHeaderKey(parts[0], headerLookup) {
			lines[i] = strings.TrimSpace(parts[0]) + ": " + redactedValue
			changed = true
		}
	}
	if changed {
		return strings.Join(lines, "\n")
	}

	return redactionPairRegex.ReplaceAllString(raw, `${1}`+redactedValue)
}

func redactQueryParamsSerialized(raw string, fieldLookup map[string]struct{}) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}

	var params map[string][]string
	if err := json.Unmarshal([]byte(trimmed), &params); err == nil {
		for key := range params {
			if isSensitiveFieldKey(key, fieldLookup) {
				params[key] = []string{redactedValue}
			}
		}
		encoded, err := json.Marshal(params)
		if err == nil {
			return string(encoded)
		}
	}

	return redactionPairRegex.ReplaceAllString(raw, `${1}`+redactedValue)
}

func redactBodySerialized(raw string, fieldLookup map[string]struct{}) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}

	var payload interface{}
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
		redacted := redactJSONValue(payload, fieldLookup)
		encoded, err := json.Marshal(redacted)
		if err == nil {
			return string(encoded)
		}
	}

	return redactionPairRegex.ReplaceAllString(raw, `${1}`+redactedValue)
}

func redactJSONValue(input interface{}, fieldLookup map[string]struct{}) interface{} {
	switch typed := input.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			if isSensitiveFieldKey(key, fieldLookup) {
				out[key] = redactedValue
				continue
			}
			out[key] = redactJSONValue(value, fieldLookup)
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		for _, value := range typed {
			out = append(out, redactJSONValue(value, fieldLookup))
		}
		return out
	default:
		return input
	}
}

func redactURLSensitiveQuery(rawURL string, fieldLookup map[string]struct{}) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return rawURL
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return rawURL
	}

	query := parsed.Query()
	changed := false
	for key := range query {
		if isSensitiveFieldKey(key, fieldLookup) {
			query.Set(key, redactedValue)
			changed = true
		}
	}
	if !changed {
		return rawURL
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func isSensitiveHeaderKey(key string, lookup map[string]struct{}) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	_, exists := lookup[normalized]
	return exists
}

func isSensitiveFieldKey(key string, lookup map[string]struct{}) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return false
	}
	if _, exists := lookup[normalized]; exists {
		return true
	}
	for token := range lookup {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	return false
}

func triggerSentRequestAlert(sent models.SentRequest) {
	webhookURL, minStatus, onError := alertSettingsSnapshot()
	if strings.TrimSpace(webhookURL) == "" {
		return
	}

	statusTrigger := sent.ResponseStatus >= minStatus && minStatus >= 100
	errorTrigger := onError && strings.TrimSpace(sent.Error) != ""
	if !statusTrigger && !errorTrigger {
		return
	}

	payload := map[string]interface{}{
		"event":           "sent_request_alert",
		"id":              sent.ID,
		"type":            sent.Type,
		"method":          sent.Method,
		"url":             sent.URL,
		"response_status": sent.ResponseStatus,
		"duration_ms":     sent.DurationMs,
		"error":           sent.Error,
		"created_at":      sent.CreatedAt.Format(time.RFC3339),
	}

	go func(target string, body map[string]interface{}) {
		if err := ValidateHTTPOutboundURL(target); err != nil {
			log.Printf("failed to deliver alert webhook: %v", err)
			return
		}

		encoded, err := json.Marshal(body)
		if err != nil {
			return
		}

		req, err := http.NewRequest(http.MethodPost, target, bytes.NewBuffer(encoded))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")

		client := newOutboundHTTPClient(5 * time.Second)
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("failed to deliver alert webhook: %v", err)
			return
		}
		_ = resp.Body.Close()
	}(webhookURL, payload)
}
