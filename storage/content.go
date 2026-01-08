package storage

import (
	"context"
	"time"
)

var ActiveContentStore ContentStore

type ContentObject struct {
	ID         string
	Type       []string
	Properties map[string][]string
	Deleted    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ContentFilter struct {
	Type       []string
	Properties map[string]string
	Deleted    *bool
	Limit      int
	Offset     int
}

type ContentStore interface {
	// function Create accepts a ContentObject and stores it, returning the URL where the
	// object can be located. If the creation fails, it will return a non-nil error.
	Create(ctx context.Context, obj *ContentObject) (string, error)

	// function Update accepts an ID that refers to an existing ContentObject, and a ContentObject.
	// The existing ContentObject is diffed with the provided ContentObject to produce a new ContentObject.
	// If the update fails, an error will be returned. Otherwise, nil is returned.
	Update(ctx context.Context, id string, obj *ContentObject) error

	// function Delete accepts an ID that refers to an existing ContentObject. If the object existed, it will
	// be marked deleted (deleted=true). It is up to the user to stop displaying a deleted object.
	Delete(ctx context.Context, id string) error

	// function Undelete accepts an ID that refers to an existing ContentObject. If the object existed, it will
	// be marked undeleted (deleted=false).
	Undelete(ctx context.Context, id string) error

	// function Get accepts an ID and returns the matching ContentObject, if any. If no object is found, a non-nil
	// error will be returned and the ContentObject pointer will be nil.
	Get(ctx context.Context, id string) (*ContentObject, error)

	// function Query accepts a ContentFilter and returns a slice of ContentObjects that match the query.
	// If an error occurs while querying, a non-nil error will be returned. The data slice will never be nil, but
	// may be empty.
	Query(ctx context.Context, filter ContentFilter) ([]*ContentObject, error)
}
