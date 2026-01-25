package util

import (
	"context"
	"fmt"
	"net/http"
)

type loggerKeyType struct{}

var loggerKey = loggerKeyType{}

// Logger is a minimal interface allowing substitution (e.g., zap, logrus).
type Logger interface {
	Printf(format string, v ...any)
}

// RequestLogger holds request-scoped context to enrich logs.
type RequestLogger struct {
	logger Logger
	method string
	path   string
	user   string
}

// WithRequest creates a request-scoped logger wrapping the provided logger.
func WithRequest(l Logger, r *http.Request, user string) *RequestLogger {
	return &RequestLogger{
		logger: l,
		method: r.Method,
		path:   r.URL.String(),
		user:   user,
	}
}

// ContextWithLogger stores the request logger in context for downstream handlers.
func ContextWithLogger(ctx context.Context, rl *RequestLogger) context.Context {
	return context.WithValue(ctx, loggerKey, rl)
}

func (rl *RequestLogger) logf(level string, message string) {
	prefix := fmt.Sprintf("%s [%s %s]", level, rl.method, rl.path)
	if rl.user != "" {
		prefix = fmt.Sprintf("%s (%s)", prefix, rl.user)
	}
	rl.logger.Printf("%s: %s", prefix, message)
}

func (rl *RequestLogger) Infof(format string, v ...any)  { rl.logf("INFO", fmt.Sprintf(format, v...)) }
func (rl *RequestLogger) Errorf(format string, v ...any) { rl.logf("ERROR", fmt.Sprintf(format, v...)) }

// FromContext retrieves a request logger from context when available.
func FromContext(ctx context.Context) *RequestLogger {
	if ctx == nil {
		return nil
	}

	if rl, ok := ctx.Value(loggerKey).(*RequestLogger); ok {
		return rl
	}

	return nil
}
