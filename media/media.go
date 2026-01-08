package media

import "net/textproto"

type UploadedFile struct {
	Filename string
	Header   textproto.MIMEHeader
	Path     string
	Size     int64
}

type MediaHandler interface {
	ProcessFile(file *UploadedFile) error
}

var Handler MediaHandler = nil

func init() {
	// Load handler
}
