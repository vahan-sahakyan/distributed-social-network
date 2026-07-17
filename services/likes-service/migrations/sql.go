package migrations

import _ "embed"

//go:embed 001_create_likes.sql
var SQL string
