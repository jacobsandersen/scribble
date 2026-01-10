package upload

import (
	"fmt"
	"net/http"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/util"
	"github.com/indieinfra/scribble/storage/media"
)

func HandleMediaUpload(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, ok := util.RequireValidMediaContentType(w, r)
		if !ok {
			return
		}

		maxSize := int64(cfg.Server.Limits.MaxFileSize)
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

		url, err := media.ActiveMediaStore.Upload(r.Context(), &file, header)
		if err != nil {
			resp.WriteInternalServerError(w, fmt.Sprintf("Error while uploading media: %v", err))
			return
		}

		resp.WriteCreated(w, url)
	}
}
