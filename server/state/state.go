package state

import (
	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/storage/content"
	"github.com/indieinfra/scribble/storage/media"
	"github.com/indieinfra/scribble/storage/util"
)

type ScribbleState struct {
	Cfg                *config.Config
	ContentPathPattern *util.PathPattern
	MediaPathPattern   *util.PathPattern
	ContentStore       content.Store
	MediaStore         media.Store
}
