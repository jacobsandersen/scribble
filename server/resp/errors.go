package resp

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/indieinfra/scribble/server/util"
	"github.com/indieinfra/scribble/storage/content"
)

// LogAndWriteError logs an error with request context and maps known conditions to client responses.
func LogAndWriteError(w http.ResponseWriter, r *http.Request, op string, err error) {
	rl := util.FromContext(r.Context())
	if rl == nil {
		rl = util.WithRequest(log.Default(), r, "")
	}
	rl.Errorf("%s failed: %v", op, err)

	switch {
	case errors.Is(err, content.ErrNotFound):
		WriteNotFound(w, "not found")
	default:
		WriteInternalServerError(w, fmt.Sprintf("%s failed", op))
	}
}
