package grpcserver

import "testing"

func TestClamp(t *testing.T) {
	tests := []struct {
		v             int32
		def, max, out int
	}{
		{0, 20, 50, 20},
		{-3, 20, 50, 20},
		{7, 20, 50, 7},
		{500, 20, 50, 50},
	}
	for _, tt := range tests {
		if got := clamp(tt.v, tt.def, tt.max); got != tt.out {
			t.Errorf("clamp(%d, %d, %d) = %d, want %d", tt.v, tt.def, tt.max, got, tt.out)
		}
	}
}
