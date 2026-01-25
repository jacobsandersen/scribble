package common

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/util"
	"github.com/indieinfra/scribble/storage/content"
)

// LogAndWriteError logs an error with request context and maps known conditions to client responses.
func LogAndWriteError(w http.ResponseWriter, r *http.Request, op string, err error) {
	rl := util.FromContext(r.Context())
	if rl == nil {
		rl = util.WithRequest(log.Default(), r, "")
	}
	rl.Errorf("micropub %s failed: %v", op, err)

	// Map known errors to user-friendly responses.
	switch {
	case errors.Is(err, content.ErrNotFound):
		resp.WriteNotFound(w, "not found")
	default:
		resp.WriteInternalServerError(w, fmt.Sprintf("%s failed", op))
	}
}
