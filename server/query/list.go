package query

import (
	"net/http"

	"github.com/indieinfra/scribble/server/body"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/storage/content"
)

func HandleList(st *state.ScribbleState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := body.ReadQueryParams(r)

		filter := content.QueryDocumentsFilter{
			Slug:           body.GetFirstOrNil(&params, "slug"),
			Category:       body.GetFirstOrNil(&params, "category"),
			CreatedYear:    body.GetIntOrNil[int64](&params, "year"),
			CreatedMonth:   body.GetIntOrNil[int64](&params, "month"),
			CreatedDay:     body.GetIntOrNil[int64](&params, "day"),
			CreatedWeekday: body.GetIntOrNil[int64](&params, "weekday"),
		}

		page := body.GetIntOrDefault(&params, "page", 1)
		if page < 1 {
			page = 1
		}

		perPage := st.Cfg.Content.Pagination.PerPage
		limit := body.GetIntOrDefault(&params, "limit", perPage)
		if limit < 1 || limit > perPage {
			limit = perPage
		}

		results, err := st.ContentStore.Query(r.Context(), page, limit, filter)
		if err != nil {
			resp.LogAndWriteError(w, r, "query documents", err)
			return
		}

		resp.WriteOK(w, results)
	}
}
