package auth

import (
	"net/http"
)

type Scope int

const (
	ScopeRead Scope = iota
	ScopeCreate
	ScopeDraft
	ScopeUpdate
	ScopeDelete
	ScopeUndelete
	ScopeMedia
)

var scopeName = map[Scope]string{
	ScopeCreate: "create",
	ScopeDraft:  "draft",
	ScopeUpdate: "update",
	ScopeDelete: "delete",
	ScopeMedia:  "media",
}

func (scope Scope) String() string {
	return scopeName[scope]
}

func RequestHasScope(r *http.Request, scope Scope) bool {
	token := GetToken(r.Context())
	if token == nil {
		return false
	}

	return token.HasScope(scope)
}
