package get

import (
	"net/http"

	"github.com/indieinfra/scribble/server/body"
	"github.com/indieinfra/scribble/server/handler/common"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
)

func HandleCategory(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, p body.QueryParams) {
	page := p.GetIntOrDefault("page", 1)
	if page < 1 {
		page = 1
	}

	perPage := st.Cfg.Content.Pagination.PerPage
	limit := p.GetIntOrDefault("limit", perPage)
	if limit < 1 || limit > perPage {
		limit = perPage
	}

	filter := p.GetFirst("filter")

	categories, err := st.ContentStore.ListCategories(r.Context(), page, limit, filter)
	if err != nil {
		common.LogAndWriteError(w, r, "list categories", err)
		return
	}

	resp.WriteOK(w, categories)
}
