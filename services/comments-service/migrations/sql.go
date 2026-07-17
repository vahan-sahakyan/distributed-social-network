package migrations

import _ "embed"

//go:embed 001_create_comments.sql
var SQL string
