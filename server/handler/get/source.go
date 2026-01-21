package get

import (
	"net/http"

	"github.com/indieinfra/scribble/server/handler/common"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/server/util"
)

func HandleSource(st *state.ScribbleState, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	url := q.Get("url")
	if url == "" {
		resp.WriteInvalidRequest(w, "Source query requires a URL parameter")
		return
	}

	doc, err := st.ContentStore.Get(r.Context(), url)
	if err != nil {
		common.LogAndWriteError(w, r, "get content", err)
		return
	}

	props := q["properties"]
	if len(props) == 0 {
		resp.WriteOK(w, doc)
		return
	}

	filtered := &util.Mf2Document{Type: doc.Type, Properties: map[string][]any{}}
	for _, p := range props {
		if vals, ok := doc.Properties[p]; ok {
			filtered.Properties[p] = vals
		}
	}

	resp.WriteOK(w, filtered)
}
