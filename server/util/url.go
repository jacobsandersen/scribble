package util

import "strings"

func UrlIsSupported(publicUrl string, url string) bool {
	return strings.HasPrefix(url, publicUrl)
}
