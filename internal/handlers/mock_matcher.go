package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"inspector/internal/models"
)

type resolvedResponse struct {
	status      int
	contentType string
	headers     map[string]string
	body        string
	delayMs     int
	ruleID      uint
}

func resolveResponse(endpoint models.Endpoint, rules []models.MockRule, req *http.Request, body []byte) resolvedResponse {
	if len(rules) > 0 {
		sorted := make([]models.MockRule, 0, len(rules))
		for _, r := range rules {
			if r.IsActive {
				sorted = append(sorted, r)
			}
		}
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].Priority == sorted[j].Priority {
				if scopeRank(sorted[i].Scope) == scopeRank(sorted[j].Scope) {
					return sorted[i].ID < sorted[j].ID
				}
				return scopeRank(sorted[i].Scope) < scopeRank(sorted[j].Scope)
			}
			return sorted[i].Priority < sorted[j].Priority
		})
		for _, rule := range sorted {
			if isGlobalRuleExcludedForEndpoint(rule, endpoint.ID) {
				continue
			}
			if matchRule(rule, req, body) {
				return buildResponse(
					rule.ResponseStatus,
					rule.ResponseHeaders,
					rule.ResponseBody,
					rule.DelayMs,
					rule.ID,
				)
			}
		}
	}

	return buildResponse(
		endpoint.ResponseStatus,
		endpoint.ResponseHeaders,
		endpoint.ResponseBody,
		0,
		0,
	)
}

func isGlobalRuleExcludedForEndpoint(rule models.MockRule, endpointID uint) bool {
	if normalizeMockScope(rule.Scope) != models.MockScopeGlobal {
		return false
	}
	raw := strings.TrimSpace(rule.ExcludedEndpointIDs)
	if raw == "" {
		return false
	}

	var ids []uint
	if err := json.Unmarshal([]byte(raw), &ids); err == nil {
		for _, id := range ids {
			if id == endpointID {
				return true
			}
		}
		return false
	}

	parts := strings.Split(raw, ",")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		parsed, err := strconv.ParseUint(trimmed, 10, 64)
		if err != nil {
			continue
		}
		if uint(parsed) == endpointID {
			return true
		}
	}

	return false
}

func scopeRank(scope string) int {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case models.MockScopeEndpoint, "":
		return 0
	case models.MockScopeGlobal:
		return 1
	default:
		return 2
	}
}

func buildResponse(status int, rawHeaders, responseBody string, delayMs int, ruleID uint) resolvedResponse {
	if status < 100 || status > 599 {
		status = 200
	}

	headers := parseHeaderMap(rawHeaders)
	contentType := "application/json"
	for k, v := range headers {
		if strings.EqualFold(k, "Content-Type") && strings.TrimSpace(v) != "" {
			contentType = strings.TrimSpace(v)
			break
		}
	}

	body := responseBody
	if strings.TrimSpace(body) == "" {
		body = `{"status":"received"}`
	}

	if delayMs < 0 {
		delayMs = 0
	}

	return resolvedResponse{
		status:      status,
		contentType: contentType,
		headers:     headers,
		body:        body,
		delayMs:     delayMs,
		ruleID:      ruleID,
	}
}

func parseHeaderMap(raw string) map[string]string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]string{}
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return map[string]string{}
	}
	if out == nil {
		return map[string]string{}
	}
	return out
}

func matchRule(rule models.MockRule, req *http.Request, body []byte) bool {
	if !matchMethod(rule.Method, req.Method) {
		return false
	}
	if !matchPath(rule.PathMode, rule.PathValue, req.URL.Path) {
		return false
	}
	if !matchQuery(rule.QueryMode, rule.QueryJSON, req.URL.Query()) {
		return false
	}
	if !matchHeaders(rule.HeadersMode, rule.HeadersJSON, req.Header) {
		return false
	}
	if !matchBody(rule.BodyMode, rule.BodyPattern, body) {
		return false
	}
	return true
}

func matchMethod(ruleMethod, reqMethod string) bool {
	rm := strings.TrimSpace(strings.ToUpper(ruleMethod))
	if rm == "" || rm == "*" || rm == "ANY" {
		return true
	}
	return rm == strings.ToUpper(reqMethod)
}

func matchPath(mode, value, path string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "any":
		return true
	case "exact":
		return path == value
	case "prefix":
		return strings.HasPrefix(path, value)
	case "regex":
		re, err := regexp.Compile(value)
		if err != nil {
			return false
		}
		return re.MatchString(path)
	default:
		return false
	}
}

func matchQuery(mode, raw string, query url.Values) bool {
	expected, ok := parseStringMap(raw)
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "any":
		return true
	case "contains":
		for k, v := range expected {
			if query.Get(k) != v {
				return false
			}
		}
		return true
	case "exact":
		if len(query) != len(expected) {
			return false
		}
		for k, v := range expected {
			if query.Get(k) != v {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func matchHeaders(mode, raw string, headers http.Header) bool {
	expected, ok := parseStringMap(raw)
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "any":
		return true
	case "contains":
		for k, v := range expected {
			if headers.Get(k) != v {
				return false
			}
		}
		return true
	case "exact":
		if len(headers) != len(expected) {
			return false
		}
		for k, v := range expected {
			if headers.Get(k) != v {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func matchBody(mode, pattern string, body []byte) bool {
	bodyStr := string(body)
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "any":
		return true
	case "exact":
		return bodyStr == pattern
	case "contains":
		return strings.Contains(bodyStr, pattern)
	case "regex":
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.Match(body)
	case "json":
		expectedRaw := strings.TrimSpace(pattern)
		if expectedRaw == "" {
			return false
		}
		var expected map[string]interface{}
		if err := json.Unmarshal([]byte(expectedRaw), &expected); err != nil {
			return false
		}
		var actual map[string]interface{}
		if err := json.Unmarshal(body, &actual); err != nil {
			return false
		}
		for k, v := range expected {
			if !jsonValueEqual(actual[k], v) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func parseStringMap(raw string) (map[string]string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]string{}, true
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, false
	}
	if out == nil {
		out = map[string]string{}
	}
	return out, true
}

func jsonValueEqual(actual interface{}, expected interface{}) bool {
	ab, errA := json.Marshal(actual)
	eb, errE := json.Marshal(expected)
	if errA != nil || errE != nil {
		return false
	}
	return string(ab) == string(eb)
}
