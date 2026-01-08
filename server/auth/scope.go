package auth

type Scope int

const (
	Read Scope = iota
	Create
	Draft
	Update
	Delete
	Undelete
	Media
)

var scopeName = map[Scope]string{
	Create: "create",
	Draft:  "draft",
	Update: "update",
	Delete: "delete",
	Media:  "media",
}

func (scope Scope) String() string {
	return scopeName[scope]
}
