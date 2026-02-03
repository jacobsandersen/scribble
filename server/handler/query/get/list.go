package get

import (
	"net/http"

	"github.com/indieinfra/scribble/server/body"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
)

func HandleList(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, p body.QueryParams) {
	resp.WriteNotFound(w, "yeehaw")
}
