package log

import "testing"

func TestLevel_UnmarshalText_RoundTrips(t *testing.T) {
	tests := []struct {
		text string
		want Level
	}{
		{"error", LevelError},
		{"warn", LevelWarn},
		{"info", LevelInfo},
		{"debug", LevelDebug},
		{"trace", LevelTrace},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			var got Level
			if err := got.UnmarshalText([]byte(tt.text)); err != nil {
				t.Fatalf("UnmarshalText(%q) error = %v", tt.text, err)
			}
			if got != tt.want {
				t.Fatalf("UnmarshalText(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestLevel_UnmarshalText_RejectsUnknown(t *testing.T) {
	var level Level
	if err := level.UnmarshalText([]byte("verbose")); err == nil {
		t.Fatal("UnmarshalText(verbose) error = nil, want error")
	}
}

func TestFormat_UnmarshalText_RoundTrips(t *testing.T) {
	tests := []struct {
		text string
		want Format
	}{
		{"text", FormatText},
		{"json", FormatJSON},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			var got Format
			if err := got.UnmarshalText([]byte(tt.text)); err != nil {
				t.Fatalf("UnmarshalText(%q) error = %v", tt.text, err)
			}
			if got != tt.want {
				t.Fatalf("UnmarshalText(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestFormat_UnmarshalText_RejectsUnknown(t *testing.T) {
	var format Format
	if err := format.UnmarshalText([]byte("yaml")); err == nil {
		t.Fatal("UnmarshalText(yaml) error = nil, want error")
	}
}

func TestLevelRange_ReturnsInclusiveBounds(t *testing.T) {
	lo, hi := LevelRange()
	if lo != LevelError || hi != LevelTrace {
		t.Fatalf("LevelRange() = (%v, %v), want (%v, %v)", lo, hi, LevelError, LevelTrace)
	}
	if !lo.Valid() || !hi.Valid() {
		t.Fatal("LevelRange() bounds must be Valid")
	}
	if (lo - 1).Valid() || (hi + 1).Valid() {
		t.Fatal("levels outside the range must be invalid")
	}
}

func TestFormatRange_ReturnsInclusiveBounds(t *testing.T) {
	lo, hi := FormatRange()
	if lo != FormatText || hi != FormatJSON {
		t.Fatalf("FormatRange() = (%v, %v), want (%v, %v)", lo, hi, FormatText, FormatJSON)
	}
	if !lo.Valid() || !hi.Valid() {
		t.Fatal("FormatRange() bounds must be Valid")
	}
	if (lo - 1).Valid() || (hi + 1).Valid() {
		t.Fatal("formats outside the range must be invalid")
	}
}

func TestLevel_Symbol_CoversAllLevelsAndInvalid(t *testing.T) {
	seen := map[string]bool{}
	for level := levelMin; level <= levelMax; level++ {
		sym := level.Symbol()
		if sym == "?" {
			t.Fatalf("Symbol(%v) = %q, want a defined badge", level, sym)
		}
		seen[sym] = true
	}
	if len(seen) != int(levelMax-levelMin+1) {
		t.Fatalf("Symbol badges not unique: %v", seen)
	}
	var invalid Level
	if got := invalid.Symbol(); got != "?" {
		t.Fatalf("Symbol(invalid) = %q, want %q", got, "?")
	}
}

func TestLevel_Allows_RespectsValidityAndOrder(t *testing.T) {
	if !LevelInfo.Allows(LevelError) {
		t.Fatal("info handler must allow error records")
	}
	if LevelWarn.Allows(LevelDebug) {
		t.Fatal("warn handler must not allow debug records")
	}
	var invalid Level
	if invalid.Allows(LevelError) {
		t.Fatal("invalid handler level must allow nothing")
	}
	if LevelInfo.Allows(invalid) {
		t.Fatal("invalid record level must be allowed by nothing")
	}
}
