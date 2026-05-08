package handlers

import (
	"bufio"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"
)

func formatJSONForDisplay(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	var payload interface{}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return raw
	}

	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return raw
	}

	return string(encoded)
}

func formatHeadersForDisplay(raw string) string {
	return formatJSONForDisplay(raw)
}

func formatBodyForDisplay(rawBody, rawHeaders string) string {
	trimmedBody := strings.TrimSpace(rawBody)
	if trimmedBody == "" {
		return rawBody
	}

	if mediaType, params, ok := parseContentTypeFromHeaders(rawHeaders); ok {
		switch {
		case isJSONMediaType(mediaType):
			return formatJSONForDisplay(rawBody)
		case mediaType == "application/x-www-form-urlencoded":
			if formatted, ok := formatURLEncodedBody(rawBody); ok {
				return formatted
			}
		case mediaType == "multipart/form-data":
			if formatted, ok := formatMultipartFormBody(rawBody, params["boundary"]); ok {
				return formatted
			}
		}
	}

	if formattedJSON := formatJSONForDisplay(rawBody); strings.TrimSpace(formattedJSON) != strings.TrimSpace(rawBody) {
		return formattedJSON
	}

	if looksLikeMultipartBody(rawBody) {
		if formatted, ok := formatMultipartFormBody(rawBody, detectMultipartBoundaryFromBody(rawBody)); ok {
			return formatted
		}
	}

	if looksLikeURLEncodedBody(rawBody) {
		if formatted, ok := formatURLEncodedBody(rawBody); ok {
			return formatted
		}
	}

	return rawBody
}

func parseContentTypeFromHeaders(rawHeaders string) (string, map[string]string, bool) {
	trimmed := strings.TrimSpace(rawHeaders)
	if trimmed == "" {
		return "", nil, false
	}

	candidates := make([]string, 0, 2)

	var multi map[string][]string
	if err := json.Unmarshal([]byte(trimmed), &multi); err == nil {
		for key, values := range multi {
			if !strings.EqualFold(strings.TrimSpace(key), "Content-Type") {
				continue
			}
			for _, value := range values {
				if candidate := strings.TrimSpace(value); candidate != "" {
					candidates = append(candidates, candidate)
				}
			}
		}
	}

	if len(candidates) == 0 {
		var single map[string]string
		if err := json.Unmarshal([]byte(trimmed), &single); err == nil {
			for key, value := range single {
				if strings.EqualFold(strings.TrimSpace(key), "Content-Type") {
					if candidate := strings.TrimSpace(value); candidate != "" {
						candidates = append(candidates, candidate)
					}
				}
			}
		}
	}

	if len(candidates) == 0 {
		return "", nil, false
	}

	mediaType, params, err := mime.ParseMediaType(candidates[0])
	if err == nil {
		return strings.ToLower(strings.TrimSpace(mediaType)), params, true
	}

	raw := strings.TrimSpace(candidates[0])
	if raw == "" {
		return "", nil, false
	}

	parts := strings.Split(raw, ";")
	base := strings.ToLower(strings.TrimSpace(parts[0]))
	fallbackParams := map[string]string{}
	for _, segment := range parts[1:] {
		entry := strings.TrimSpace(segment)
		if entry == "" {
			continue
		}
		kv := strings.SplitN(entry, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		value := strings.Trim(strings.TrimSpace(kv[1]), "\"")
		if key != "" && value != "" {
			fallbackParams[key] = value
		}
	}

	if base == "" {
		return "", nil, false
	}

	return base, fallbackParams, true
}

func isJSONMediaType(mediaType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(mediaType))
	if normalized == "application/json" || normalized == "text/json" {
		return true
	}
	return strings.HasSuffix(normalized, "+json")
}

func formatURLEncodedBody(rawBody string) (string, bool) {
	parsed, err := url.ParseQuery(strings.TrimSpace(rawBody))
	if err != nil || len(parsed) == 0 {
		return "", false
	}

	keys := make([]string, 0, len(parsed))
	for key := range parsed {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make(map[string]interface{}, len(keys))
	for _, key := range keys {
		values := parsed[key]
		if len(values) == 1 {
			out[key] = values[0]
			continue
		}
		out[key] = values
	}

	encoded, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", false
	}

	return string(encoded), true
}

type multipartFileDisplay struct {
	Field       string `json:"field"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType,omitempty"`
	SizeBytes   int    `json:"sizeBytes"`
	Preview     string `json:"preview,omitempty"`
}

func formatMultipartFormBody(rawBody, boundary string) (string, bool) {
	cleanBoundary := strings.Trim(strings.TrimSpace(boundary), "\"")
	if cleanBoundary == "" {
		cleanBoundary = detectMultipartBoundaryFromBody(rawBody)
	}
	if cleanBoundary == "" {
		return "", false
	}

	reader := multipart.NewReader(strings.NewReader(rawBody), cleanBoundary)
	fieldValues := map[string][]string{}
	files := make([]multipartFileDisplay, 0)

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", false
		}

		payload, err := io.ReadAll(part)
		if err != nil {
			return "", false
		}

		name := strings.TrimSpace(part.FormName())
		if name == "" {
			name = "part"
		}

		filename := strings.TrimSpace(part.FileName())
		if filename != "" {
			entry := multipartFileDisplay{
				Field:     name,
				Filename:  filename,
				SizeBytes: len(payload),
			}
			if contentType := strings.TrimSpace(part.Header.Get("Content-Type")); contentType != "" {
				entry.ContentType = contentType
			}
			if preview, ok := asReadablePreview(payload); ok && preview != "" {
				entry.Preview = preview
			}
			files = append(files, entry)
			continue
		}

		value := strings.TrimSpace(string(payload))
		if value == "" {
			value = string(payload)
		}
		fieldValues[name] = append(fieldValues[name], value)
	}

	if len(fieldValues) == 0 && len(files) == 0 {
		return "", false
	}

	fieldKeys := make([]string, 0, len(fieldValues))
	for key := range fieldValues {
		fieldKeys = append(fieldKeys, key)
	}
	sort.Strings(fieldKeys)

	cleanFields := make(map[string]interface{}, len(fieldKeys))
	for _, key := range fieldKeys {
		values := fieldValues[key]
		if len(values) == 1 {
			cleanFields[key] = values[0]
		} else {
			cleanFields[key] = values
		}
	}

	if len(files) == 0 {
		encoded, err := json.MarshalIndent(cleanFields, "", "  ")
		if err != nil {
			return "", false
		}
		return string(encoded), true
	}

	wrapper := map[string]interface{}{
		"fields": cleanFields,
		"files":  files,
	}

	encoded, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return "", false
	}

	return string(encoded), true
}

func detectMultipartBoundaryFromBody(rawBody string) string {
	scanner := bufio.NewScanner(strings.NewReader(rawBody))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "--") {
			boundary := strings.TrimPrefix(line, "--")
			boundary = strings.TrimSuffix(boundary, "--")
			boundary = strings.TrimSpace(boundary)
			if boundary != "" {
				return boundary
			}
		}
	}
	return ""
}

func looksLikeMultipartBody(rawBody string) bool {
	trimmed := strings.TrimSpace(rawBody)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "Content-Disposition: form-data") {
		return true
	}

	for _, line := range strings.Split(trimmed, "\n") {
		candidate := strings.TrimSpace(line)
		if candidate == "" {
			continue
		}
		return strings.HasPrefix(candidate, "--") && len(candidate) > 4
	}

	return false
}

func looksLikeURLEncodedBody(rawBody string) bool {
	trimmed := strings.TrimSpace(rawBody)
	if trimmed == "" {
		return false
	}
	if strings.ContainsAny(trimmed, "{}[]\n\r") {
		return false
	}
	return strings.Contains(trimmed, "=")
}

func asReadablePreview(payload []byte) (string, bool) {
	if len(payload) == 0 {
		return "", true
	}
	if !utf8.Valid(payload) {
		return "", false
	}

	text := strings.TrimSpace(string(payload))
	if text == "" {
		text = string(payload)
	}
	if !isMostlyReadable(text) {
		return "", false
	}

	runes := []rune(text)
	if len(runes) > 300 {
		text = string(runes[:300]) + "..."
	}

	return text, true
}

func isMostlyReadable(text string) bool {
	if text == "" {
		return true
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return true
	}

	invalid := 0
	for _, r := range runes {
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if r < 32 {
			invalid++
		}
	}

	return float64(invalid)/float64(len(runes)) <= 0.05
}
