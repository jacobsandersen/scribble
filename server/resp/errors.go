package resp

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/jacobsandersen/scribble/server/util"
)

// LogAndWriteError logs an error with request context and maps known conditions to client responses.
func LogAndWriteError(w http.ResponseWriter, r *http.Request, op string, err error) {
	rl := util.FromContext(r.Context())
	if rl == nil {
		rl = util.WithRequest(log.Default(), r, "")
	}
	rl.Errorf("%s failed: %v", op, err)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		WriteNotFound(w, "not found")
	default:
		WriteInternalServerError(w, fmt.Sprintf("%s failed", op))
	}
}
