package handlers

import (
	"strings"
	"testing"
)

func TestFormatBodyForDisplay_JSON(t *testing.T) {
	headers := `{"Content-Type":["application/json"]}`
	body := `{"ok":true,"count":2}`

	formatted := formatBodyForDisplay(body, headers)

	if !strings.Contains(formatted, "\n") {
		t.Fatalf("expected formatted JSON with new lines, got: %q", formatted)
	}
	if !strings.Contains(formatted, `"ok": true`) {
		t.Fatalf("expected formatted JSON to preserve values, got: %q", formatted)
	}
}

func TestFormatBodyForDisplay_URLEncoded(t *testing.T) {
	headers := `{"Content-Type":["application/x-www-form-urlencoded"]}`
	body := "name=monkey&role=inspector"

	formatted := formatBodyForDisplay(body, headers)

	if !strings.Contains(formatted, `"name": "monkey"`) {
		t.Fatalf("expected urlencoded body to be converted to JSON, got: %q", formatted)
	}
	if !strings.Contains(formatted, `"role": "inspector"`) {
		t.Fatalf("expected urlencoded body to include all keys, got: %q", formatted)
	}
}

func TestFormatBodyForDisplay_Multipart(t *testing.T) {
	boundary := "------------------------d74496d66958873e"
	headers := `{"Content-Type":["multipart/form-data; boundary=` + boundary + `"]}`
	body := "--" + boundary + "\r\n" +
		"Content-Disposition: form-data; name=\"monkey\"\r\n\r\n" +
		"macaco\r\n" +
		"--" + boundary + "--\r\n"

	formatted := formatBodyForDisplay(body, headers)

	if !strings.Contains(formatted, `"monkey": "macaco"`) {
		t.Fatalf("expected multipart body to be cleaned into fields JSON, got: %q", formatted)
	}
	if strings.Contains(formatted, "Content-Disposition") {
		t.Fatalf("expected multipart metadata to be removed, got: %q", formatted)
	}
}

func TestFormatBodyForDisplay_MultipartBoundaryDetectionWithoutHeaders(t *testing.T) {
	boundary := "--------------------------9051914041544843365972754266"
	body := "--" + boundary + "\n" +
		"Content-Disposition: form-data; name=\"a\"\n\n" +
		"b\n" +
		"--" + boundary + "--\n"

	formatted := formatBodyForDisplay(body, "")

	if !strings.Contains(formatted, `"a": "b"`) {
		t.Fatalf("expected multipart boundary autodetection to work, got: %q", formatted)
	}
}

func TestFormatBodyForDisplay_LeavesPlainTextUntouched(t *testing.T) {
	body := "hola mundo"
	formatted := formatBodyForDisplay(body, `{"Content-Type":["text/plain"]}`)

	if formatted != body {
		t.Fatalf("expected plain text body unchanged, got: %q", formatted)
	}
}
