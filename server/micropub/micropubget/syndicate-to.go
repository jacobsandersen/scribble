package micropubget

import (
	"net/http"

	"github.com/jacobsandersen/scribble/server/body"
	"github.com/jacobsandersen/scribble/server/resp"
	"github.com/jacobsandersen/scribble/server/state"
)

func HandleSyndicateTo(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, p body.QueryParams) {
	// TODO: Implement syndicate-to retrieval
	resp.WriteOK(w, map[string]any{
		"syndicate-to": []any{},
	})
}
