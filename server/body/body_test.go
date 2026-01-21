package body

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/indieinfra/scribble/config"
)

func testBodyConfig() *config.Config {
	return &config.Config{
		Server: config.Server{
			Limits: config.ServerLimits{
				MaxPayloadSize:  1024,
				MaxFileSize:     512,
				MaxMultipartMem: 2048,
			},
		},
	}
}

func TestReadJSONInvalid(t *testing.T) {
	cfg := testBodyConfig()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"invalid":`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	if got := readJSON(cfg, rr, req); got != nil {
		t.Fatalf("expected invalid JSON to return nil body")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Invalid JSON body") {
		t.Fatalf("expected invalid request response, got %q", rr.Body.String())
	}
}

func TestReadBodyFormPopsAccessToken(t *testing.T) {
	cfg := testBodyConfig()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("h=entry&category=go&category=web&access_token=secret"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	parsed, ok := ReadBody(cfg, rr, req)
	if !ok {
		t.Fatalf("expected form body to parse")
	}
	if parsed.AccessToken != "secret" {
		t.Fatalf("expected access token to be popped, got %q", parsed.AccessToken)
	}
	if _, exists := parsed.Data["access_token"]; exists {
		t.Fatalf("expected access_token to be removed from data")
	}
	cats, ok := parsed.Data["category"].([]any)
	if !ok || len(cats) != 2 || cats[0] != "go" || cats[1] != "web" {
		t.Fatalf("expected category slice with values, got %#v", parsed.Data["category"])
	}
	if parsed.Data["h"] != "entry" {
		t.Fatalf("expected h=entry, got %#v", parsed.Data["h"])
	}
}

func TestReadFormURLEncodedLimitExceeded(t *testing.T) {
	cfg := testBodyConfig()
	cfg.Server.Limits.MaxPayloadSize = 4

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("title=toolong"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	if got := readFormURLEncoded(cfg, rr, req); got != nil {
		t.Fatalf("expected nil body when form exceeds limit")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid request status, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Invalid form body") {
		t.Fatalf("expected invalid form body message, got %q", rr.Body.String())
	}
}

func TestReadMultipartSuccess(t *testing.T) {
	cfg := testBodyConfig()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("h", "entry")
	_ = w.WriteField("access_token", "multipart-token")
	fw, _ := w.CreateFormFile("photo", "pic.jpg")
	_, _ = fw.Write([]byte("abc"))
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()

	body, ok := readMultipart(cfg, rr, req)
	if !ok || body == nil {
		t.Fatalf("expected multipart body to parse")
	}
	if body.AccessToken != "multipart-token" {
		t.Fatalf("expected access token to be returned, got %q", body.AccessToken)
	}
	if _, exists := body.Data["access_token"]; exists {
		t.Fatalf("expected access_token to be removed from multipart data")
	}
	if body.Data["h"] != "entry" {
		t.Fatalf("expected h=entry in multipart data, got %#v", body.Data["h"])
	}
	if len(body.Files) != 1 || body.Files[0].Field != "photo" {
		t.Fatalf("expected photo file to be captured, got %#v", body.Files)
	}
	body.Files[0].File.Close()
}

func TestReadMultipartFileTooLarge(t *testing.T) {
	cfg := testBodyConfig()
	cfg.Server.Limits.MaxFileSize = 1

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("photo", "pic.jpg")
	_, _ = fw.Write([]byte("toolarge"))
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()

	if body, ok := readMultipart(cfg, rr, req); ok || body != nil {
		t.Fatalf("expected multipart parsing to fail for large file")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid multipart status, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "failed to read multipart form") {
		t.Fatalf("expected multipart error response, got %q", rr.Body.String())
	}
}
