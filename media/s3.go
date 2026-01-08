package media

type S3MediaHandler struct {
	Endpoint    string
	Region      string
	AccessKeyId string
	SecretKeyId string
	Bucket      string
}

func (h *S3MediaHandler) ProcessFile(f *UploadedFile) error {
	return nil
}
