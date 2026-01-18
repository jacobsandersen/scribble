package common

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/indieinfra/scribble/storage/content"
)

func TestLogAndWriteInternal_MapsTo500(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	LogAndWriteInternal(rr, req, "op", errors.New("boom"))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestLogAndWriteInternal_NotFound(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	LogAndWriteInternal(rr, req, "op", content.ErrNotFound)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
