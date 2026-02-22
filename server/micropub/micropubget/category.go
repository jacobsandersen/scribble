package micropubget

import (
	"net/http"

	"github.com/jacobsandersen/scribble/server/body"
	"github.com/jacobsandersen/scribble/server/resp"
	"github.com/jacobsandersen/scribble/server/state"
)

func HandleCategory(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, p body.QueryParams) {
	page := body.GetIntOrDefault(&p, "page", 1)
	if page < 1 {
		page = 1
	}

	perPage := st.Cfg.Content.Pagination.PerPage
	limit := body.GetIntOrDefault(&p, "limit", perPage)
	if limit < 1 || limit > perPage {
		limit = perPage
	}

	filter := body.GetFirst(&p, "filter")

	categories, err := st.ContentStore.ListCategories(r.Context(), page, limit, filter)
	if err != nil {
		resp.LogAndWriteError(w, r, "list categories", err)
		return
	}

	resp.WriteOK(w, categories)
}
