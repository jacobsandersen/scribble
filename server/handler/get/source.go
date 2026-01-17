package get

import (
	"net/http"

	"github.com/indieinfra/scribble/server/handler/common"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
)

func HandleSource(st *state.ScribbleState, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	url := q.Get("url")
	if url == "" {
		resp.WriteInvalidRequest(w, "source requires a url")
		return
	}

	doc, err := st.ContentStore.Get(r.Context(), url)
	if err != nil {
		common.LogAndWriteError(w, r, "get content", err)
		return
	}

	resp.WriteOK(w, doc)
}
