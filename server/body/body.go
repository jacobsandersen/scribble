package body

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/auth"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/util"
)

// ParsedBody represents the parsed content from an HTTP request body,
// including data fields, uploaded files, and access token (if present in body).
type ParsedBody struct {
	Data        map[string]any
	Files       []ParsedFile
	AccessToken string
}

// ParsedFile represents a file uploaded as part of a multipart request.
type ParsedFile struct {
	File   multipart.File
	Header *multipart.FileHeader
	Field  string
}

// ReadBody parses the request body based on content type (JSON, form-urlencoded, or multipart).
// Returns the parsed body and true on success, or nil and false on failure.
// Writes appropriate error responses directly to the ResponseWriter on failure.
func ReadBody(cfg *config.Config, w http.ResponseWriter, r *http.Request) (*ParsedBody, bool) {
	_, contentType, ok := util.RequireValidMicropubContentType(w, r)
	if !ok {
		return nil, false
	}

	switch contentType {
	case "application/json":
		data := readJSON(cfg, w, r)
		if data == nil {
			return nil, false
		}
		return &ParsedBody{Data: data}, true
	case "application/x-www-form-urlencoded":
		data := readFormURLEncoded(cfg, w, r)
		if data == nil {
			return nil, false
		}
		token := auth.PopAccessToken(data)
		return &ParsedBody{Data: data, AccessToken: token}, true
	case "multipart/form-data":
		return readMultipart(cfg, w, r)
	}

	return nil, false
}

// readJSON parses a JSON request body.
func readJSON(cfg *config.Config, w http.ResponseWriter, r *http.Request) map[string]any {
	out := make(map[string]any)

	r.Body = http.MaxBytesReader(w, r.Body, int64(cfg.Server.Limits.MaxPayloadSize))
	if err := json.NewDecoder(r.Body).Decode(&out); err != nil {
		resp.WriteInvalidRequest(w, "Invalid JSON body")
		return nil
	}

	return out
}

// readFormURLEncoded parses an application/x-www-form-urlencoded request body.
func readFormURLEncoded(cfg *config.Config, w http.ResponseWriter, r *http.Request) map[string]any {
	out := make(map[string]any)

	r.Body = http.MaxBytesReader(w, r.Body, int64(cfg.Server.Limits.MaxPayloadSize))
	if err := r.ParseForm(); err != nil {
		resp.WriteInvalidRequest(w, fmt.Sprintf("Invalid form body: %v", err))
		return nil
	}

	for key, values := range r.Form {
		switch len(values) {
		case 0:
			continue
		case 1:
			out[key] = values[0]
		default:
			arr := make([]any, len(values))
			for i, v := range values {
				arr[i] = v
			}
			out[key] = arr
		}
	}

	return out
}

// readMultipart parses a multipart/form-data request body, extracting both
// form fields and uploaded files.
func readMultipart(cfg *config.Config, w http.ResponseWriter, r *http.Request) (*ParsedBody, bool) {
	maxMemory := int64(cfg.Server.Limits.MaxMultipartMem)
	maxFile := int64(cfg.Server.Limits.MaxFileSize)
	fields := []string{"photo", "video", "audio", "file"}
	values, files, ok := util.ParseMultipartFiles(w, r, maxMemory, maxFile, fields, false)
	if !ok {
		return nil, false
	}

	token := auth.PopAccessToken(values)

	parsedFiles := make([]ParsedFile, 0, len(files))
	for _, f := range files {
		parsedFiles = append(parsedFiles, ParsedFile{File: f.File, Header: f.Header, Field: f.Field})
	}

	return &ParsedBody{Data: values, Files: parsedFiles, AccessToken: token}, true
}
