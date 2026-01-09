package post

import (
	"fmt"
	"net/http"

	"github.com/indieinfra/scribble/server/auth"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/storage/content"
)

func Delete(w http.ResponseWriter, r *http.Request, data map[string]any, isUndelete bool) {
	urlRaw, ok := data["url"]
	if !ok {
		resp.WriteInvalidRequest(w, "URL to (un)delete must be specified")
		return
	}

	url, ok := urlRaw.(string)
	if !ok {
		resp.WriteInvalidRequest(w, "URL to delete must be a string")
		return
	}

	var err error = nil
	if isUndelete {
		if !auth.RequestHasScope(r, auth.ScopeUndelete) {
			resp.WriteInsufficientScope(w, "no undelete scope")
			return
		}

		url, isNewUrl, err2 := content.ActiveContentStore.Undelete(r.Context(), url)
		if err2 != nil {
			resp.WriteInternalServerError(w, fmt.Sprintf("Error during undeletion: %v", err))
		} else if isNewUrl {
			resp.WriteCreated(w, url)
		} else {
			resp.WriteNoContent(w)
		}
	} else {
		if !auth.RequestHasScope(r, auth.ScopeDelete) {
			resp.WriteInsufficientScope(w, "no delete scope")
			return
		}

		err2 := content.ActiveContentStore.Delete(r.Context(), url)
		if err2 != nil {
			resp.WriteInternalServerError(w, fmt.Sprintf("Error during deletion: %v", err))
		} else {
			resp.WriteNoContent(w)
		}
	}
}
