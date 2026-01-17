package util

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubLogger struct{ messages []string }

func (s *stubLogger) Printf(format string, v ...any) {
	s.messages = append(s.messages, fmt.Sprintf(format, v...))
}

func TestRequestLoggerPrefixes(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mp", nil)
	logger := &stubLogger{}
	rl := WithRequest(logger, req, "user-123")

	rl.Infof("hello %s", "world")
	rl.Errorf("oops %d", 500)

	if len(logger.messages) != 2 {
		t.Fatalf("expected 2 log messages, got %d", len(logger.messages))
	}
	if msg := logger.messages[0]; !strings.HasPrefix(msg, "INFO") {
		t.Fatalf("expected INFO prefix, got %q", msg)
	}
	if msg := logger.messages[1]; !strings.HasPrefix(msg, "ERROR") {
		t.Fatalf("expected ERROR prefix, got %q", msg)
	}
	if !containsAll(logger.messages[0], []string{"method=POST", "path=/mp", "user=user-123", "hello world"}) {
		t.Fatalf("expected structured fields in info log, got %q", logger.messages[0])
	}
}

func TestContextWithLoggerRoundTrip(t *testing.T) {
	t.Run("stores and retrieves logger", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		logger := &stubLogger{}
		rl := WithRequest(logger, req, "")

		ctx := ContextWithLogger(context.Background(), rl)
		got := FromContext(ctx)
		if got != rl {
			t.Fatalf("expected to retrieve same logger from context")
		}
	})

	t.Run("returns nil when logger absent", func(t *testing.T) {
		if FromContext(context.Background()) != nil {
			t.Fatalf("expected background context without logger to return nil")
		}
	})

	t.Run("ignores non-logger values", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), loggerKey, "not-a-logger")
		if FromContext(ctx) != nil {
			t.Fatalf("expected non-logger value to return nil")
		}
	})
}

func containsAll(s string, parts []string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
