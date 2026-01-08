package media

type SftpMediaHandler struct {
	address   string
	port      uint
	username  string
	password  string
	keyfile   string
	directory string
}

func (h *SftpMediaHandler) ProcessFile(f *UploadedFile) error {
	return nil
}
