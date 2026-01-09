package upload

import (
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/util"
	"github.com/indieinfra/scribble/storage/media"
)

func HandleMediaUpload(w http.ResponseWriter, r *http.Request) {
	_, _, ok := util.RequireValidMediaContentType(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, int64(config.MaxMultipartSize()))

	mr, err := r.MultipartReader()
	if err != nil {
		resp.WriteInvalidRequest(w, fmt.Sprintf("Invalid multipart body: %v", err))
		return
	}

	file, err := readMultipartBody(mr)
	if err != nil {
		resp.WriteInternalServerError(w, err.Error())
		return
	}

	url, err := media.ActiveMediaStore.Upload(r.Context(), file)
	if err != nil {
		resp.WriteInternalServerError(w, fmt.Sprintf("Error while uploading media: %v", err))
		return
	}

	resp.WriteCreated(w, url)
}

func readMultipartBody(mr *multipart.Reader) (*media.UploadedFile, error) {
	for {
		part, err := mr.NextPart()
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, fmt.Errorf("Failed to read multipart body: %w", err)
		}

		if part.FormName() == "" {
			log.Printf("warning: skipping unnamed multipart part: filename=%q", part.FileName())
			part.Close()
			continue
		}

		if part.FileName() != "" {
			fh, err := streamFilePart(part)
			if err != nil {
				return nil, fmt.Errorf("Failed to stream file part: %w", err)
			}

			return fh, nil
		}
	}

	return nil, errors.New("Did not find a file in the multipart request")
}

func streamFilePart(part *multipart.Part) (*media.UploadedFile, error) {
	defer part.Close()

	limit := int64(config.MaxFileSize())

	tmp, err := os.CreateTemp("", "scribble-upload-*")
	if err != nil {
		return nil, fmt.Errorf("Failed to store file upload: %w", err)
	}

	defer tmp.Close()

	n, err := io.Copy(tmp, io.LimitReader(part, limit+1))
	if err != nil {
		return nil, fmt.Errorf("Failed to read file upload: %w", err)
	}

	if n > limit {
		return nil, fmt.Errorf("Uploaded file exceeds maximum file size (%v > %v bytes)", n, limit)
	}

	fh := &media.UploadedFile{
		Filename: part.FileName(),
		Header:   part.Header,
		Path:     tmp.Name(),
		Size:     n,
	}

	return fh, nil
}
