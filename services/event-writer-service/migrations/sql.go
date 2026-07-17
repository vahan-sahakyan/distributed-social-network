package migrations

import _ "embed"

//go:embed 001_create_feed_events.sql
var SQL string
