package util

import (
	"log"
	"mime"
	"mime/multipart"
	"net/http"
)

type MultipartValues map[string]any

type MultipartFile struct {
	Field            string
	File             multipart.File
	Header           *multipart.FileHeader
	DetectedMimeType string
	FileExtension    string
}

type ParsedMultipart struct {
	Values MultipartValues
	Files  []*MultipartFile
}

func (pm *ParsedMultipart) CloseFiles() {
	for _, mf := range pm.Files {
		if mf.File != nil {
			mf.File.Close()
		}
	}
}

func (pm *ParsedMultipart) FileByKey(key string) *MultipartFile {
	for _, mf := range pm.Files {
		if mf.Field == key {
			return mf
		}
	}

	return nil
}

func ParseMultipart(w http.ResponseWriter, r *http.Request, maxMemory, maxFileSize int64) (*ParsedMultipart, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		return nil, err
	}

	values := extractValues(r)
	files := extractFiles(r, maxFileSize)

	return &ParsedMultipart{
		Values: values,
		Files:  files,
	}, nil
}

func extractValues(r *http.Request) MultipartValues {
	values := make(MultipartValues)

	if r.MultipartForm != nil {
		for key, arr := range r.MultipartForm.Value {
			switch len(arr) {
			case 0:
				continue
			case 1:
				values[key] = arr[0]
			default:
				asAny := make([]any, len(arr))
				for i, v := range arr {
					asAny[i] = v
				}
				values[key] = asAny
			}
		}
	}

	return values
}

func extractFiles(r *http.Request, maxFileSize int64) []*MultipartFile {
	var filesOut []*MultipartFile

	for key, fhs := range r.MultipartForm.File {
		for _, fh := range fhs {
			mf := processFileHeader(fh, maxFileSize)
			if mf == nil {
				continue
			}

			mf.Field = key

			filesOut = append(filesOut, mf)
		}
	}

	return filesOut
}

func processFileHeader(fh *multipart.FileHeader, maxFileSize int64) *MultipartFile {
	if maxFileSize > 0 && fh.Size > maxFileSize {
		log.Println("skipped too large file:", fh.Filename, fh.Size)
		return nil
	}

	f, err := fh.Open()
	if err != nil {
		log.Println("skipped file, could not open:", fh.Filename, err)
		return nil
	}

	mimeType, err := detectMimeType(f)
	if err != nil {
		log.Println("skipped file, could not detect mime type:", fh.Filename, err)
		f.Close()
		return nil
	}

	ext, err := suggestFileExtension(mimeType)
	if err != nil {
		log.Println("skipped file, could not suggest file extension:", fh.Filename, mimeType, err)
		f.Close()
		return nil
	}

	return &MultipartFile{File: f, Header: fh, DetectedMimeType: mimeType, FileExtension: ext}
}

func detectMimeType(file multipart.File) (string, error) {
	buffer := make([]byte, 512)
	_, err := file.Read(buffer)
	if err != nil {
		return "", err
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return "", err
	}

	mimeType := http.DetectContentType(buffer)
	return mimeType, nil
}

func suggestFileExtension(mimeType string) (string, error) {
	exts, err := mime.ExtensionsByType(mimeType)
	if err != nil || len(exts) == 0 {
		return "", err
	}

	return exts[0], nil
}
