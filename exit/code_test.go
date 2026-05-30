package exit

import "testing"

type coder int

func (c coder) ExitCode() int { return int(c) }

func TestIsError_RecognizesDefinedNonZeroCodes(t *testing.T) {
	tests := []struct {
		name string
		code int
		want bool
	}{
		{"ok", OK, false},
		{"min sentinel", _min, false},
		{"usage", Usage, true},
		{"software", Software, true},
		{"config", Config, true},
		{"max sentinel", _max, false},
		{"below range", _min - 10, false},
		{"above range", _max + 10, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsError(coder(tt.code)); got != tt.want {
				t.Fatalf("IsError(%d) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestExitCodes_AreContiguousAndOrdered(t *testing.T) {
	if Usage != _min+1 {
		t.Fatalf("Usage = %d, want %d", Usage, _min+1)
	}
	if _max <= Config {
		t.Fatalf("_max = %d, want greater than Config = %d", _max, Config)
	}
	if OK != 0 {
		t.Fatalf("OK = %d, want 0", OK)
	}
}
