package repository

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/vahan-sahakyan/distributed-social-network/users-service/internal/model"
)

func TestTranslate(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantTaken bool
	}{
		{"wrapped unique violation", fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23505"}), true},
		{"not null violation", &pgconn.PgError{Code: "23502"}, false},
		{"non pg error", errors.New("boom"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := translate(tt.err)
			if errors.Is(got, model.ErrUsernameTaken) != tt.wantTaken {
				t.Errorf("translate(%v) = %v, want taken=%v", tt.err, got, tt.wantTaken)
			}
			if !tt.wantTaken && !errors.Is(got, tt.err) {
				t.Errorf("translate(%v) = %v, want it unchanged", tt.err, got)
			}
		})
	}
	if translate(nil) != nil {
		t.Error("translate(nil) != nil")
	}
}
