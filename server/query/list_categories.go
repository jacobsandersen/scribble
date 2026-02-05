package query

import (
	"net/http"

	"github.com/indieinfra/scribble/server/body"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
)

func HandleListCategories(st *state.ScribbleState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := body.ReadQueryParams(r)

		filter := body.GetFirstOrDefault(&params, "filter", "")

		page := body.GetIntOrDefault(&params, "page", 1)
		if page < 1 {
			page = 1
		}

		perPage := st.Cfg.Content.Pagination.PerPage
		limit := body.GetIntOrDefault(&params, "limit", perPage)
		if limit < 1 || limit > perPage {
			limit = perPage
		}

		results, err := st.ContentStore.ListCategories(r.Context(), page, limit, filter)
		if err != nil {
			resp.LogAndWriteError(w, r, "query categories", err)
			return
		}

		resp.WriteOK(w, results)
	}
}
