package migrations

import _ "embed"

//go:embed 001_create_posts.sql
var SQL string
