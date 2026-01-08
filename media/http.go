package media

type HttpMediaHandler struct {
	url     string
	headers map[string]string
}

func (h *HttpMediaHandler) ProcessFile(f *UploadedFile) error {
	return nil
}
