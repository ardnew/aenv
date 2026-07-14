package log

import (
	"log/slog"
	"testing"
)

func TestEncodeJSONEvent_NestsUserAttrsUnderAttr(t *testing.T) {
	record := eventRecord{
		level: LevelInfo,
		timestamp: reprVariant[string]{
			long: fixedTestTime.Format(longTimestampLayout),
		},
		whence: callsite{
			scope: "log.TestEncodeJSONEvent_NestsUserAttrsUnderAttr",
			position: reprVariant[string]{
				long: "log/encode_test.go:1",
			},
		},
		message: "hello",
		attrs: normalizeEventAttrs([]slog.Attr{
			slog.Group("ctx", slog.Int("id", 7), slog.String("name", "svc")),
			slog.String("scope", "user-scope"),
			slog.String("ctx.id", "literal"),
		}),
	}

	got := string(encodeJSONEvent(targetFile, record))
	want := "{\"time\":\"2026-05-10T12:34:56.789+0200\",\"level\":\"info\",\"source\":\"log/encode_test.go:1\",\"scope\":\"log.TestEncodeJSONEvent_NestsUserAttrsUnderAttr\",\"attr\":{\"ctx\":{\"id\":7,\"name\":\"svc\"},\"scope\":\"user-scope\",\"ctx.id\":\"literal\"},\"message\":\"hello\"}\n"
	if got != want {
		t.Fatalf("encodeJSONEvent() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestEncodeJSONEvent_OmitsEmptyBuiltinsAndAttr(t *testing.T) {
	record := eventRecord{
		level: LevelInfo,
		timestamp: reprVariant[string]{
			long: fixedTestTime.Format(longTimestampLayout),
		},
		message: "hello",
	}

	got := string(encodeJSONEvent(targetFile, record))
	want := "{\"time\":\"2026-05-10T12:34:56.789+0200\",\"level\":\"info\",\"message\":\"hello\"}\n"
	if got != want {
		t.Fatalf("encodeJSONEvent() omission mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestEncodeTextEvent_NamespacesUserAttrs(t *testing.T) {
	record := eventRecord{
		level: LevelDebug,
		timestamp: reprVariant[string]{
			short: fixedTestTime.Format(shortTimestampLayout),
		},
		whence: callsite{
			scope: "log.TestEncodeTextEvent_NamespacesUserAttrs",
			position: reprVariant[string]{
				short: "encode_test.go:1",
			},
		},
		message: "hello",
		attrs: normalizeEventAttrs([]slog.Attr{
			slog.Group("ctx", slog.Int("id", 7)),
			slog.String("scope", "user-scope"),
			slog.String("ctx.id", "literal"),
		}),
	}

	got := string(encodeTextEvent(targetTerminal, LevelDebug, record))
	want := "12:34:56.789 · source=encode_test.go:1 scope=log.TestEncodeTextEvent_NamespacesUserAttrs attr.ctx.id=7 attr.scope=user-scope attr.ctx.id=literal :: hello\r\n"
	if got != want {
		t.Fatalf("encodeTextEvent() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}
