package media

import (
	"context"

	"github.com/jacobsandersen/scribble/server/util"
)

type Store interface {
	Upload(ctx context.Context, data *util.MultipartFile, key string) (err error)
	Delete(ctx context.Context, url string) (err error)
}
