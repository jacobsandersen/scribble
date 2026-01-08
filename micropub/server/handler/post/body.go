package post

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/micropub/resp"
	"github.com/indieinfra/scribble/micropub/server/util"
)

type UploadedFile struct {
	Filename string
	Header   textproto.MIMEHeader
	Path     string
	Size     int64
}

type MicropubData struct {
	Properties map[string]any
	Files      map[string][]*UploadedFile
}

func ReadBody(w http.ResponseWriter, r *http.Request) *MicropubData {
	_, contentType, ok := util.RequireValidContentType(w, r)
	if !ok {
		return nil
	}

	data := &MicropubData{
		Properties: make(map[string]any),
		Files:      make(map[string][]*UploadedFile),
	}

	switch contentType {
	case "application/json":
		return readJsonBody(w, r, data)
	case "application/x-www-form-urlencoded":
		return readFormUrlEncodedBody(w, r, data)
	case "multipart/form-data":
		return readMultipartBody(w, r, data)
	}

	return nil
}

func readJsonBody(w http.ResponseWriter, r *http.Request, d *MicropubData) *MicropubData {
	r.Body = http.MaxBytesReader(w, r.Body, int64(config.MaxPayloadSize()))
	if err := json.NewDecoder(r.Body).Decode(&d.Properties); err != nil {
		resp.WriteHttpError(w, http.StatusBadRequest, fmt.Errorf("Invalid JSON body: %w", err).Error())
		return nil
	}

	return d
}

func readFormUrlEncodedBody(w http.ResponseWriter, r *http.Request, d *MicropubData) *MicropubData {
	r.Body = http.MaxBytesReader(w, r.Body, int64(config.MaxPayloadSize()))
	if err := r.ParseForm(); err != nil {
		resp.WriteHttpError(w, http.StatusUnprocessableEntity, fmt.Errorf("Invalid form body: %w", err).Error())
		return nil
	}

	for key, values := range r.Form {
		switch len(values) {
		case 0:
			continue
		case 1:
			d.Properties[key] = values[0]
		default:
			arr := make([]any, len(values))
			for i, v := range values {
				arr[i] = v
			}
			d.Properties[key] = arr
		}
	}

	return d
}

func readMultipartBody(w http.ResponseWriter, r *http.Request, d *MicropubData) *MicropubData {
	r.Body = http.MaxBytesReader(w, r.Body, int64(config.MaxMultipartSize()))

	mr, err := r.MultipartReader()
	if err != nil {
		resp.WriteHttpError(w, http.StatusUnprocessableEntity, fmt.Errorf("Invalid multipart body: %w", err).Error())
		return nil
	}

	for {
		part, err := mr.NextPart()
		if err != nil {
			if err == io.EOF {
				break
			}

			resp.WriteHttpError(w, http.StatusBadRequest, fmt.Errorf("Failed to read multipart body: %w", err).Error())
			return nil
		}

		name := part.FormName()
		if name == "" {
			log.Printf("warning: skipping unnamed multipart part: filename=%q", part.FileName())
			part.Close()
			continue
		}

		if part.FileName() != "" {
			fh, err := streamFilePart(part)
			if err != nil {
				// error written in helper
				return nil
			}
			d.Files[name] = append(d.Files[name], fh)
			continue
		}

		value, err := readFormPart(part)
		if err != nil {
			resp.WriteHttpError(w, http.StatusUnprocessableEntity, "Invalid multipart form field")
			return nil
		}

		value = strings.TrimSpace(value)

		existing, ok := d.Properties[name]
		if !ok {
			d.Properties[name] = value
			continue
		}

		switch v := existing.(type) {
		case []any:
			d.Properties[name] = append(v, value)
		default:
			d.Properties[name] = []any{v, value}
		}
	}

	return d
}

func streamFilePart(part *multipart.Part) (*UploadedFile, error) {
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

	fh := &UploadedFile{
		Filename: part.FileName(),
		Header:   part.Header,
		Path:     tmp.Name(),
		Size:     n,
	}

	return fh, nil
}

func readFormPart(part *multipart.Part) (string, error) {
	defer part.Close()

	data, err := io.ReadAll(part)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
