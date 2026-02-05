package query

import (
	"net/http"

	"github.com/indieinfra/scribble/server/body"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
)

func HandleFind(st *state.ScribbleState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := body.ReadQueryParams(r)
		slug := body.GetFirstOrNil(&params, "slug")
		if slug == nil {
			resp.WriteBadRequest(w, "missing required parameter: slug")
			return
		}

		result, err := st.ContentStore.GetBySlug(r.Context(), *slug)
		if err != nil {
			resp.LogAndWriteError(w, r, "find document by slug", err)
			return
		}

		resp.WriteOK(w, result)
	}
}
