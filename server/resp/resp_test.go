package resp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteOK(t *testing.T) {
	rr := httptest.NewRecorder()

	WriteOK(rr, map[string]string{"hello": "world"})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json content type, got %q", ct)
	}

	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["hello"] != "world" {
		t.Fatalf("unexpected body %v", body)
	}
}

func TestWriteCreatedAddsLocation(t *testing.T) {
	rr := httptest.NewRecorder()

	WriteCreated(rr, "/posts/123")

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	if rr.Header().Get("Location") != "/posts/123" {
		t.Fatalf("expected Location header set")
	}
	if body := rr.Body.String(); body != "" {
		t.Fatalf("expected empty body, got %q", body)
	}
}

func TestWriteBadRequest(t *testing.T) {
	rr := httptest.NewRecorder()

	WriteBadRequest(rr, "bad input")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	var body ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body.Error != "bad request" || body.Description != "bad input" {
		t.Fatalf("unexpected error response %+v", body)
	}
}

func TestWriteErrorVariants(t *testing.T) {
	cases := []struct {
		name  string
		write func(http.ResponseWriter)
		code  int
		err   string
		desc  string
	}{
		{
			name:  "unauthorized",
			write: func(w http.ResponseWriter) { WriteUnauthorized(w, "need token") },
			code:  http.StatusUnauthorized, err: "unauthorized", desc: "need token",
		},
		{
			name:  "forbidden",
			write: func(w http.ResponseWriter) { WriteForbidden(w, "no access") },
			code:  http.StatusForbidden, err: "forbidden", desc: "no access",
		},
		{
			name:  "insufficient",
			write: func(w http.ResponseWriter) { WriteInsufficientScope(w, "need scope") },
			code:  http.StatusUnauthorized, err: "insufficient_scope", desc: "need scope",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			tc.write(rr)

			if rr.Code != tc.code {
				t.Fatalf("expected %d, got %d", tc.code, rr.Code)
			}

			var body ErrorResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to decode body: %v", err)
			}
			if body.Error != tc.err || body.Description != tc.desc {
				t.Fatalf("unexpected body %+v", body)
			}
		})
	}
}

func TestWriteNoContentAndAccepted(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteNoContent(rr)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("expected empty body")
	}

	rr = httptest.NewRecorder()
	WriteAccepted(rr, "/jobs/1")
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
	if rr.Header().Get("Location") != "/jobs/1" {
		t.Fatalf("expected Location header")
	}
}

func TestWriteInvalidRequest(t *testing.T) {
	rr := httptest.NewRecorder()

	WriteInvalidRequest(rr, "bad")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	var body ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body.Error != "invalid_request" || body.Description != "bad" {
		t.Fatalf("unexpected body %+v", body)
	}
}

func TestWriteInternalServerError(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteInternalServerError(rr, "oops")

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestWriteNotFound(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteNotFound(rr, "missing")

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
