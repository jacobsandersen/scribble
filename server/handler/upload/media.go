package upload

import (
	"fmt"
	"net/http"

	"github.com/indieinfra/scribble/server/handler/common"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/server/util"
)

func HandleMediaUpload(st *state.ScribbleState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, ok := util.RequireValidMediaContentType(w, r)
		if !ok {
			return
		}

		maxSize := int64(st.Cfg.Server.Limits.MaxFileSize)
		r.Body = http.MaxBytesReader(w, r.Body, maxSize)
		err := r.ParseMultipartForm(maxSize)
		if err != nil {
			resp.WriteInvalidRequest(w, fmt.Sprintf("failed to read multipart form: %v", err))
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			resp.WriteInvalidRequest(w, fmt.Sprintf("failed to find file data in request: %v", err))
			return
		}

		defer file.Close()

		url, err := st.MediaStore.Upload(r.Context(), &file, header)
		if err != nil {
			common.LogAndWriteError(w, r, "upload media", err)
			return
		}

		resp.WriteCreated(w, url)
	}
}
