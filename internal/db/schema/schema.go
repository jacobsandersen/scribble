package schema

// Embed is required to include the schema SQL file in the compiled binary.
import _ "embed"

//go:embed sqlite.sql
var Sqlite string
