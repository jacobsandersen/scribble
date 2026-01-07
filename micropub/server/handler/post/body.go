package post

import (
	"net/http"

	"github.com/indieinfra/scribble/micropub/server/util"
)

type MicropubBody struct {
	action string
}

func ReadBody(w http.ResponseWriter, r *http.Request) *MicropubBody {
	_, contentType, ok := util.RequireValidContentType(w, r)
	if !ok {
		return nil
	}

	switch contentType {
	case "application/json":
		return readJsonBody(w, r)
	case "application/x-www-form-urlencoded":
		return readFormUrlEncodedBody(w, r)
	case "multipart/form-data":
		return readMultipartBody(w, r)
	}

	return nil
}

func readJsonBody(w http.ResponseWriter, r *http.Request) *MicropubBody {
	return nil
}

func readFormUrlEncodedBody(w http.ResponseWriter, r *http.Request) *MicropubBody {
	return nil
}

func readMultipartBody(w http.ResponseWriter, r *http.Request) *MicropubBody {
	return nil
}
