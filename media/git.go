package media

type GitMediaHandler struct {
	repository string
	username   string
	password   string
	directory  string
}

func (h *GitMediaHandler) ProcessFile(f *UploadedFile) error {
	return nil
}
