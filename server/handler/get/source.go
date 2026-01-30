package get

import (
	"net/http"
	"slices"

	"github.com/indieinfra/scribble/server/body"
	"github.com/indieinfra/scribble/server/handler/common"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/server/util"
)

func HandleSource(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, p body.QueryParams) {
	urlParam := p.Get("url")
	if urlParam == nil {
		handleMany(st, w, r, p)
	} else {
		url := urlParam.Value
		if len(url) == 0 {
			resp.WriteInvalidRequest(w, "No URL found")
			return
		}

		if !util.UrlIsSupported(st.Cfg.Content.PublicBaseUrl, url[0]) {
			resp.WriteInvalidRequest(w, "Invalid URL (not a supported destination)")
			return
		}

		handleOne(st, w, r, p, url[0])
	}
}

func handleMany(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, p body.QueryParams) {
	page := p.GetIntOrDefault("page", 1)
	if page < 1 {
		page = 1
	}

	perPage := st.Cfg.Content.Pagination.PerPage
	limit := p.GetIntOrDefault("limit", perPage)
	if limit < 1 || limit > perPage {
		limit = perPage
	}

	docs, err := st.ContentStore.List(r.Context(), page, limit)
	if err != nil {
		common.LogAndWriteError(w, r, "list content", err)
		return
	}

	resp.WriteOK(w, filterDocs(docs, p.Get("properties")))
}

func handleOne(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, p body.QueryParams, url string) {
	doc, err := st.ContentStore.Get(r.Context(), url)
	if err != nil {
		common.LogAndWriteError(w, r, "get content", err)
		return
	}

	resp.WriteOK(w, filterDoc(*doc, p.Get("properties")))
}

func filterDocs(docs []util.Mf2Document, properties *body.QueryParam) []any {
	out := make([]any, 0, len(docs))

	for _, doc := range docs {
		filtered := filterDoc(doc, properties)
		if filtered != nil {
			out = append(out, filtered)
		}
	}

	return out
}

func filterDoc(doc util.Mf2Document, properties *body.QueryParam) any {
	if properties == nil {
		return doc
	}

	outProps := make(util.MicroformatProperties)
	for key, _ := range doc.Properties {
		if slices.Contains(properties.Value, key) {
			outProps[key] = doc.Properties[key]
		}
	}

	if len(outProps) == 0 {
		return nil
	}

	return outProps
}
