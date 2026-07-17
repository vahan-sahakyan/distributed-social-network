package migrations

import _ "embed"

//go:embed 001_create_notifications.sql
var SQL string
