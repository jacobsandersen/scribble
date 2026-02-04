package body

import (
	"net/http"
	"strconv"
	"strings"
)

// QueryParam represents a single query parameter with one key mapping to potentially many values
type QueryParam struct {
	Key   string
	Value []string
}

// QueryParams represents all query parameters for a URL. Bracketed keys are collapsed to their non-bracketed
// equivalents. That is, key properties[] == key properties. For a query parameter set ?properties[]=a&properties=b,
// this struct will contain one QueryParam with key=properties and value=[a,b].
type QueryParams struct {
	Params []QueryParam
}

type intType interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

func intBitSize[T intType]() int {
	var zero T
	switch any(zero).(type) {
	case int8:
		return 8
	case int16:
		return 16
	case int32:
		return 32
	case int64:
		return 64
	default:
		return strconv.IntSize
	}
}

// Get gets a single QueryParam from the given QueryParams
func Get(p *QueryParams, key string) *QueryParam {
	for i := range p.Params {
		if p.Params[i].Key == key {
			return &p.Params[i]
		}
	}

	return nil
}

// GetFirst gets the first value for a QueryParam from the given QueryParams
// If the key does not map a param, or there are no values, an empty string is returned
func GetFirst(p *QueryParams, key string) string {
	param := Get(p, key)
	if param == nil || len(param.Value) == 0 {
		return ""
	}

	return param.Value[0]
}

// GetFirstOrNil gets the first value for a QueryParam from the given QueryParams
// If the key does not map a param, or there are no values, nil is returned
func GetFirstOrNil(p *QueryParams, key string) *string {
	first := GetFirst(p, key)
	if first == "" {
		return nil
	}

	return &first
}

// GetIntOrNil finds a single QueryParam from the QueryParams and attempts to parse its first value as an int-like type
// If successful, a pointer to that value is returned. Otherwise, nil is returned.
func GetIntOrNil[T intType](p *QueryParams, key string) *T {
	first := GetFirst(p, key)
	if first == "" {
		return nil
	}

	if tmp, err := strconv.ParseInt(first, 10, intBitSize[T]()); err == nil {
		val := T(tmp)
		return &val
	}

	return nil
}

// GetIntOrDefault finds a single QueryParam from the QueryParams and attempts to parse its first value as an int-like type
// If successful, that value is returned. Otherwise, the provided default value is returned.
func GetIntOrDefault[T intType](p *QueryParams, key string, def T) T {
	maybe := GetIntOrNil[T](p, key)
	if maybe != nil {
		return *maybe
	}

	return def
}

// Add adds or appends a []string to the QueryParam that maps to the given key. If no key currently maps,
// a new QueryParam is created.
func Add(p *QueryParams, key string, value []string) {
	param := Get(p, key)
	if param == nil {
		p.Params = append(p.Params, QueryParam{key, value})
	} else {
		param.Value = append(param.Value, value...)
	}
}

func ReadQueryParams(r *http.Request) QueryParams {
	params := QueryParams{}
	for key, value := range r.URL.Query() {
		key = strings.TrimSuffix(key, "[]")
		Add(&params, key, value)
	}
	return params
}
