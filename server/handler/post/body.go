package post

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/resp"
	"github.com/indieinfra/scribble/server/util"
)

type MicropubData struct {
	Properties map[string]any
}

func (d *MicropubData) GetString(key string) (string, bool, error) {
	val, ok := d.Properties[key]
	if !ok {
		return "", false, nil
	}

	switch v := val.(type) {
	case string:
		return v, true, nil
	case []any:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return s, true, nil
			}
		}
		return "", true, errors.New("value exists, but first element is not a string")
	default:
		return "", true, errors.New("value exists, but is not a string")
	}
}

func (d *MicropubData) GetStringSlice(key string) ([]string, bool, error) {
	val, ok := d.Properties[key]
	if !ok {
		return nil, false, nil
	}

	switch v := val.(type) {
	case string:
		return []string{v}, true, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, x := range v {
			s, ok := x.(string)
			if !ok {
				return nil, true, errors.New("value exists, but contains non-string elements")
			}

			out = append(out, s)
		}
		return out, true, nil
	default:
		return nil, true, errors.New("value exists, but cannot coerce to string slice")
	}
}

func ReadBody(w http.ResponseWriter, r *http.Request) *MicropubData {
	_, contentType, ok := util.RequireValidMicropubContentType(w, r)
	if !ok {
		return nil
	}

	data := &MicropubData{Properties: make(map[string]any)}

	switch contentType {
	case "application/json":
		return readJsonBody(w, r, data)
	case "application/x-www-form-urlencoded":
		return readFormUrlEncodedBody(w, r, data)
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
