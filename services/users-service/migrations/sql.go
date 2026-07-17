package migrations

import _ "embed"

//go:embed 001_create_users.sql
var SQL string
