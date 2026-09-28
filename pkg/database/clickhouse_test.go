package database

import (
	"strings"
	"testing"
)

func statements(sql string) []string {
	var out []string
	for _, stmt := range strings.Split(stripLineComments(sql), ";") {
		if stmt = strings.TrimSpace(stmt); stmt != "" {
			out = append(out, stmt)
		}
	}
	return out
}

func TestStripLineComments(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []string
	}{
		{
			name: "semicolon inside a comment does not split a statement",
			sql:  "CREATE TABLE a (x Int32);\n\n-- drop it; it is unused\nDROP VIEW IF EXISTS v;",
			want: []string{"CREATE TABLE a (x Int32)", "DROP VIEW IF EXISTS v"},
		},
		{
			name: "comment-only migration yields no statements",
			sql:  "-- nothing to run here\n-- still nothing",
			want: nil,
		},
		{
			name: "trailing comment is removed",
			sql:  "SELECT 1 -- one",
			want: []string{"SELECT 1"},
		},
		{
			name: "double dash inside a string literal is kept",
			sql:  "SELECT 'a--b' -- trailing",
			want: []string{"SELECT 'a--b'"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statements(tt.sql)
			if strings.Join(got, "|") != strings.Join(tt.want, "|") || len(got) != len(tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
