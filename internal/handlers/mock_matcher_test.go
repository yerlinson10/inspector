package handlers

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"inspector/internal/models"
)

func TestResolveResponse_FallbackEndpoint(t *testing.T) {
	endpoint := models.Endpoint{
		ResponseStatus:  201,
		ResponseHeaders: `{"Content-Type":"application/json"}`,
		ResponseBody:    `{"ok":true}`,
	}

	req := &http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/in/demo"}, Header: make(http.Header)}
	resp := resolveResponse(endpoint, nil, req, []byte(`{"a":1}`))
	if resp.status != 201 {
		t.Fatalf("expected fallback status 201, got %d", resp.status)
	}
	if resp.ruleID != 0 {
		t.Fatalf("expected no matched rule, got %d", resp.ruleID)
	}
}

func TestResolveResponse_UsesFirstMatchByPriority(t *testing.T) {
	endpoint := models.Endpoint{ResponseStatus: 200, ResponseBody: `{"from":"endpoint"}`}
	rules := []models.MockRule{
		{
			ID:             10,
			Priority:       200,
			IsActive:       true,
			Method:         "POST",
			PathMode:       "exact",
			PathValue:      "/in/demo",
			ResponseStatus: 418,
			ResponseBody:   `{"from":"low-priority"}`,
		},
		{
			ID:             11,
			Priority:       100,
			IsActive:       true,
			Method:         "POST",
			PathMode:       "exact",
			PathValue:      "/in/demo",
			ResponseStatus: 202,
			ResponseBody:   `{"from":"high-priority"}`,
		},
	}

	req := &http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/in/demo"}, Header: make(http.Header)}
	resp := resolveResponse(endpoint, rules, req, []byte(`{"event":"deploy"}`))
	if resp.status != 202 {
		t.Fatalf("expected status from highest priority rule (202), got %d", resp.status)
	}
	if resp.ruleID != 11 {
		t.Fatalf("expected matched rule 11, got %d", resp.ruleID)
	}
}

func TestResolveResponse_BodyRegexAndContains(t *testing.T) {
	endpoint := models.Endpoint{ResponseStatus: 200, ResponseBody: `{"from":"endpoint"}`}
	rules := []models.MockRule{
		{
			ID:             21,
			Priority:       10,
			IsActive:       true,
			Method:         "POST",
			PathMode:       "any",
			BodyMode:       "regex",
			BodyPattern:    `"status"\s*:\s*"ok"`,
			ResponseStatus: 209,
			ResponseBody:   `{"from":"regex"}`,
		},
	}

	req := &http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/in/demo"}, Header: make(http.Header)}
	resp := resolveResponse(endpoint, rules, req, []byte(`{"status":"ok"}`))
	if resp.status != 209 {
		t.Fatalf("expected regex rule to match, got status %d", resp.status)
	}
	if !strings.Contains(resp.body, "regex") {
		t.Fatalf("expected regex response body, got %s", resp.body)
	}
}

func TestResolveResponse_EndpointScopeWinsTieOverGlobal(t *testing.T) {
	endpoint := models.Endpoint{ResponseStatus: 200, ResponseBody: `{"from":"endpoint-default"}`}
	rules := []models.MockRule{
		{
			ID:             30,
			Scope:          models.MockScopeGlobal,
			Priority:       100,
			IsActive:       true,
			Method:         "POST",
			PathMode:       "exact",
			PathValue:      "/in/demo",
			ResponseStatus: 210,
			ResponseBody:   `{"from":"global"}`,
		},
		{
			ID:             31,
			Scope:          models.MockScopeEndpoint,
			Priority:       100,
			IsActive:       true,
			Method:         "POST",
			PathMode:       "exact",
			PathValue:      "/in/demo",
			ResponseStatus: 211,
			ResponseBody:   `{"from":"endpoint-scope"}`,
		},
	}

	req := &http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/in/demo"}, Header: make(http.Header)}
	resp := resolveResponse(endpoint, rules, req, []byte(`{"event":"deploy"}`))
	if resp.status != 211 {
		t.Fatalf("expected endpoint-scope rule to win tie, got status %d", resp.status)
	}
	if resp.ruleID != 31 {
		t.Fatalf("expected matched rule 31, got %d", resp.ruleID)
	}
}

func TestResolveResponse_GlobalRuleExcludedForEndpoint(t *testing.T) {
	endpoint := models.Endpoint{ID: 7, ResponseStatus: 200, ResponseBody: `{"from":"endpoint-default"}`}
	rules := []models.MockRule{
		{
			ID:                  40,
			Scope:               models.MockScopeGlobal,
			Priority:            10,
			IsActive:            true,
			Method:              "POST",
			PathMode:            "exact",
			PathValue:           "/in/demo",
			ExcludedEndpointIDs: `[7]`,
			ResponseStatus:      299,
			ResponseBody:        `{"from":"global"}`,
		},
	}

	req := &http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/in/demo"}, Header: make(http.Header)}
	resp := resolveResponse(endpoint, rules, req, []byte(`{"event":"deploy"}`))
	if resp.ruleID != 0 {
		t.Fatalf("expected excluded global rule to be skipped, got rule %d", resp.ruleID)
	}
	if resp.status != 200 {
		t.Fatalf("expected fallback endpoint status 200, got %d", resp.status)
	}
}
