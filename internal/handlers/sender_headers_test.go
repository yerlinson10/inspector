package handlers

import (
	"encoding/json"
	"testing"
)

func TestParseSenderHeadersJSON_AllowsStringAndStringArrayValues(t *testing.T) {
	raw := `{"Authorization":"Bearer token","Set-Cookie":["a=1","b=2"]}`

	headers, err := parseSenderHeadersJSON(raw)
	if err != nil {
		t.Fatalf("expected valid headers, got error: %v", err)
	}

	if got := len(headers["Authorization"]); got != 1 {
		t.Fatalf("expected Authorization with 1 value, got %d", got)
	}
	if headers["Authorization"][0] != "Bearer token" {
		t.Fatalf("unexpected Authorization value: %q", headers["Authorization"][0])
	}

	if got := len(headers["Set-Cookie"]); got != 2 {
		t.Fatalf("expected Set-Cookie with 2 values, got %d", got)
	}
	if headers["Set-Cookie"][0] != "a=1" || headers["Set-Cookie"][1] != "b=2" {
		t.Fatalf("unexpected Set-Cookie values: %#v", headers["Set-Cookie"])
	}
}

func TestParseSenderHeadersJSON_RejectsNonStringValues(t *testing.T) {
	raw := `{"X-Retry":1}`

	_, err := parseSenderHeadersJSON(raw)
	if err == nil {
		t.Fatalf("expected validation error for non-string header value")
	}
}

func TestReplayHeadersForSender_ProducesEditorFriendlyJSON(t *testing.T) {
	raw := `{"Connection":["keep-alive"],"Set-Cookie":["a=1","b=2"]}`

	formatted := replayHeadersForSender(raw)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(formatted), &parsed); err != nil {
		t.Fatalf("expected replay headers to be valid JSON, got error: %v; content: %q", err, formatted)
	}

	connection, ok := parsed["Connection"].(string)
	if !ok || connection != "keep-alive" {
		t.Fatalf("expected Connection to be a single string, got %#v", parsed["Connection"])
	}

	cookies, ok := parsed["Set-Cookie"].([]interface{})
	if !ok || len(cookies) != 2 {
		t.Fatalf("expected Set-Cookie to be an array with 2 values, got %#v", parsed["Set-Cookie"])
	}
}
