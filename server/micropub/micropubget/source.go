package micropubget

import (
	"net/http"
	"slices"

	"github.com/indieinfra/scribble/server/body"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/server/util"
)

func HandleSource(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, p body.QueryParams) {
	urlParam := body.Get(&p, "url")
	if urlParam == nil {
		handleMany(st, w, r, p)
	} else {
		url := urlParam.Value
		if len(url) == 0 {
			resp.WriteInvalidRequest(w, "No URL found")
			return
		}

		if !util.UrlIsInstance(st.Cfg.Content.ContentUrl, url[0]) {
			resp.WriteInvalidRequest(w, "Invalid URL (not a supported destination)")
			return
		}

		handleOne(st, w, r, p, url[0])
	}
}

func handleMany(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, p body.QueryParams) {
	page := body.GetIntOrDefault(&p, "page", 1)
	if page < 1 {
		page = 1
	}

	perPage := st.Cfg.Content.Pagination.PerPage
	limit := body.GetIntOrDefault(&p, "limit", perPage)
	if limit < 1 || limit > perPage {
		limit = perPage
	}

	docs, err := st.ContentStore.List(r.Context(), page, limit)
	if err != nil {
		resp.LogAndWriteError(w, r, "list content", err)
		return
	}

	resp.WriteOK(w, filterDocs(docs, body.Get(&p, "properties")))
}

func handleOne(st *state.ScribbleState, w http.ResponseWriter, r *http.Request, p body.QueryParams, url string) {
	doc, err := st.ContentStore.Get(r.Context(), url)
	if err != nil {
		resp.LogAndWriteError(w, r, "get content", err)
		return
	}

	resp.WriteOK(w, filterDoc(doc, body.Get(&p, "properties")))
}

func filterDocs(docs []*util.Mf2Document, properties *body.QueryParam) []any {
	out := make([]any, 0, len(docs))

	for _, doc := range docs {
		filtered := filterDoc(doc, properties)
		if filtered != nil {
			out = append(out, filtered)
		}
	}

	return out
}

func filterDoc(doc *util.Mf2Document, properties *body.QueryParam) any {
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
