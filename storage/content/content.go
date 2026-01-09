package content

import (
	"context"

	"github.com/indieinfra/scribble/server/util"
)

var ActiveContentStore ContentStore

type MicropubProperties map[string][]any

type ContentObject struct {
	Url        string
	Type       []string
	Properties MicropubProperties
	Deleted    bool
}

type ContentStore interface {
	// function Create accepts a ContentObject and stores it, returning the URL where the
	// object can be located. If the creation fails, it will return a non-nil error.
	Create(ctx context.Context, doc util.Mf2Document) (string, error)

	// function Update accepts an ID that refers to an existing ContentObject, and a ContentObject.
	// The existing ContentObject is diffed with the provided ContentObject to produce a new ContentObject.
	// If the update fails, an error will be returned. Otherwise, nil is returned.
	Update(ctx context.Context, url string, replacements map[string][]any, additions map[string][]any, deletions any) (string, error)

	// function Delete accepts an ID that refers to an existing ContentObject. If the object existed, it will
	// be marked deleted (deleted=true). It is up to the user to stop displaying a deleted object.
	Delete(ctx context.Context, url string) error

	// function Undelete accepts an ID that refers to an existing ContentObject. If the object existed, it will
	// be marked undeleted (deleted=false).
	Undelete(ctx context.Context, url string) (string, bool, error)

	// function Get accepts an ID and returns the matching ContentObject, if any. If no object is found, a non-nil
	// error will be returned and the ContentObject pointer will be nil.
	Get(ctx context.Context, url string) (*ContentObject, error)
}
