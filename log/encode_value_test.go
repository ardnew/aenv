package log

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func jsonEventWithAttr(t *testing.T, attr slog.Attr) string {
	t.Helper()
	record := eventRecord{
		level:     LevelInfo,
		timestamp: reprVariant[string]{long: fixedTestTime.Format(longTimestampLayout)},
		message:   "m",
		attrs:     normalizeEventAttrs([]slog.Attr{attr}),
	}
	return string(encodeJSONEvent(targetFile, record))
}

func textEventWithAttr(t *testing.T, attr slog.Attr) string {
	t.Helper()
	record := eventRecord{
		level:     LevelInfo,
		timestamp: reprVariant[string]{short: fixedTestTime.Format(shortTimestampLayout)},
		message:   "m",
		attrs:     normalizeEventAttrs([]slog.Attr{attr}),
	}
	return string(encodeTextEvent(targetTerminal, LevelInfo, record))
}

type stringerValue struct{ n int }

func (s stringerValue) String() string { return "S" + string(rune('0'+s.n)) }

func TestEncode_ValueKinds_JSONAndText(t *testing.T) {
	dur := 1500 * time.Millisecond
	when := fixedTestTime
	tests := []struct {
		name     string
		attr     slog.Attr
		jsonFrag string
		textFrag string
	}{
		{"bool", slog.Bool("k", true), `"k":true`, "attr.k=true"},
		{"int", slog.Int64("k", -42), `"k":-42`, "attr.k=-42"},
		{"uint", slog.Uint64("k", 7), `"k":7`, "attr.k=7"},
		{"float", slog.Float64("k", 3.5), `"k":3.5`, "attr.k=3.5"},
		{"duration", slog.Duration("k", dur), `"k":"1.5s"`, "attr.k=1.5s"},
		{"time", slog.Time("k", when), `"k":"` + when.Format(time.RFC3339Nano) + `"`, "attr.k=" + when.Format(time.RFC3339Nano)},
		{"string", slog.String("k", "a\"b"), `"k":"a\"b"`, `attr.k="a\"b"`},
		{"any", slog.Any("k", stringerValue{2}), `"k":{}`, "attr.k=S2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jsonEventWithAttr(t, tt.attr); !strings.Contains(got, tt.jsonFrag) {
				t.Fatalf("json: got %q, want fragment %q", got, tt.jsonFrag)
			}
			if got := textEventWithAttr(t, tt.attr); !strings.Contains(got, tt.textFrag) {
				t.Fatalf("text: got %q, want fragment %q", got, tt.textFrag)
			}
		})
	}
}

func TestEncode_JSONPrimitives_MatchEncodingJSON(t *testing.T) {
	values := []any{true, int64(-9), uint64(42), "html<>&", "u\u2603"}
	for _, v := range values {
		want, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("json.Marshal(%v) error = %v", v, err)
		}
		var buf bytes.Buffer
		appendJSONValue(&buf, slog.AnyValue(v))
		if got := buf.Bytes(); !bytes.Equal(got, want) {
			t.Fatalf("appendJSONValue(%v) = %s, want %s", v, got, want)
		}
	}
}
