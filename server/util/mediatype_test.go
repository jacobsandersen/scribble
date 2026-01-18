package util

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequireValidMicropubContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	rr := httptest.NewRecorder()

	_, mediaType, ok := RequireValidMicropubContentType(rr, req)
	if !ok {
		t.Fatalf("expected content type to be accepted")
	}
	if mediaType != "application/json" {
		t.Fatalf("expected media type application/json, got %q", mediaType)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status code %d", rr.Code)
	}
}

func TestRequireValidMicropubContentTypeRejectsInvalid(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	if _, _, ok := RequireValidMicropubContentType(rr, req); ok {
		t.Fatalf("expected invalid content type to be rejected")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestExtractMediaType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=abc")
	rr := httptest.NewRecorder()

	mediaType, ok := ExtractMediaType(rr, req)
	if !ok {
		t.Fatalf("expected media type to parse")
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("unexpected media type %q", mediaType)
	}
}

func TestExtractMediaTypeMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rr := httptest.NewRecorder()

	if _, ok := ExtractMediaType(rr, req); ok {
		t.Fatalf("expected missing content type to fail")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestRequireValidMediaContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=abc")
	rr := httptest.NewRecorder()

	if _, mediaType, ok := RequireValidMediaContentType(rr, req); !ok || mediaType != "multipart/form-data" {
		t.Fatalf("expected multipart/form-data to be accepted")
	}
}

func TestRequireValidMediaContentTypeRejectsInvalid(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	if _, _, ok := RequireValidMediaContentType(rr, req); ok {
		t.Fatalf("expected invalid media content type to be rejected")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestRequireValidMediaContentTypeAllowsGet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	method, mediaType, ok := RequireValidMediaContentType(rr, req)
	if !ok || method != http.MethodGet || mediaType != "" {
		t.Fatalf("expected GET requests to bypass content-type checks")
	}
}
