package state

import (
	"github.com/jacobsandersen/scribble/config"
	"github.com/jacobsandersen/scribble/storage/content"
	"github.com/jacobsandersen/scribble/storage/media"
	"github.com/jacobsandersen/scribble/storage/util"
)

type ScribbleState struct {
	Cfg                *config.Config
	ContentPathPattern *util.PathPattern
	MediaPathPattern   *util.PathPattern
	ContentStore       content.Store
	MediaStore         media.Store
}
