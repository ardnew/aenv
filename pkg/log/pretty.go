package log

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"sync"
)

// ANSI color codes for pretty printing.
const (
	colorReset   = "\033[0m"
	colorGray    = "\033[90m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
)

// prettyTextHandler implements a colorized text handler for log messages.
type prettyTextHandler struct {
	opts   slog.HandlerOptions
	mu     *sync.Mutex
	w      io.Writer
	groups []string
}

func newPrettyTextHandler(
	w io.Writer,
	opts *slog.HandlerOptions,
) *prettyTextHandler {
	return &prettyTextHandler{
		opts:   *opts,
		mu:     &sync.Mutex{},
		w:      w,
		groups: []string{},
	}
}

func (h *prettyTextHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *prettyTextHandler) Handle(_ context.Context, r slog.Record) error {
	buf := new(bytes.Buffer)

	// Write time if configured
	if !r.Time.IsZero() {
		timeAttr := slog.Time(slog.TimeKey, r.Time)
		h.writeAttr(buf, timeAttr)
	}

	// Write level
	levelAttr := slog.Any(slog.LevelKey, r.Level)
	h.writeAttr(buf, levelAttr)

	// Write source if configured
	if h.opts.AddSource {
		if src := r.Source(); src != nil {
			// Format as file:line
			sourceStr := fmt.Sprintf("%s:%d", src.File, src.Line)
			sourceAttr := slog.String(slog.SourceKey, sourceStr)
			h.writeAttr(buf, sourceAttr)
		}
	}

	// Write message
	msgAttr := slog.String(slog.MessageKey, r.Message)
	h.writeAttr(buf, msgAttr)

	// Write each attribute
	r.Attrs(func(a slog.Attr) bool {
		h.writeAttr(buf, a)

		return true
	})

	h.mu.Lock()
	defer h.mu.Unlock()

	_, err := h.w.Write(buf.Bytes())
	if err != nil {
		return err
	}

	_, err = h.w.Write([]byte("\n"))

	return err
}

func (h *prettyTextHandler) WithAttrs([]slog.Attr) slog.Handler {
	// Create a new handler with the same configuration
	return &prettyTextHandler{
		opts:   h.opts,
		mu:     h.mu,
		w:      h.w,
		groups: h.groups,
	}
}

func (h *prettyTextHandler) WithGroup(name string) slog.Handler {
	return &prettyTextHandler{
		opts:   h.opts,
		mu:     h.mu,
		w:      h.w,
		groups: append(h.groups[:len(h.groups):len(h.groups)], name),
	}
}

func (h *prettyTextHandler) writeAttr(buf *bytes.Buffer, a slog.Attr) {
	if buf.Len() > 0 {
		buf.WriteByte(' ')
	}

	// Write key in gray
	buf.WriteString(colorGray)
	buf.WriteString(a.Key)
	buf.WriteString(colorReset)
	buf.WriteByte('=')

	// Write value in color based on type
	h.writeValue(buf, a.Value)
}

func (h *prettyTextHandler) writeValue(buf *bytes.Buffer, v slog.Value) {
	switch v.Kind() {
	case slog.KindString:
		// String values in cyan, no quotes
		buf.WriteString(colorCyan)
		buf.WriteString(v.String())
		buf.WriteString(colorReset)

	case slog.KindInt64:
		// Numbers in yellow
		buf.WriteString(colorYellow)
		buf.WriteString(strconv.FormatInt(v.Int64(), 10))
		buf.WriteString(colorReset)

	case slog.KindUint64:
		buf.WriteString(colorYellow)
		buf.WriteString(strconv.FormatUint(v.Uint64(), 10))
		buf.WriteString(colorReset)

	case slog.KindFloat64:
		buf.WriteString(colorYellow)
		buf.WriteString(strconv.FormatFloat(v.Float64(), 'g', -1, 64))
		buf.WriteString(colorReset)

	case slog.KindBool:
		// Booleans in green/red
		if v.Bool() {
			buf.WriteString(colorGreen)
			buf.WriteString("true")
		} else {
			buf.WriteString(colorRed)
			buf.WriteString("false")
		}

		buf.WriteString(colorReset)

	case slog.KindDuration:
		buf.WriteString(colorMagenta)
		buf.WriteString(v.Duration().String())
		buf.WriteString(colorReset)

	case slog.KindTime:
		buf.WriteString(colorBlue)
		buf.WriteString(v.Time().String())
		buf.WriteString(colorReset)

	case slog.KindAny:
		// Handle slog.Level specially
		if level, ok := v.Any().(slog.Level); ok {
			// Color code based on level
			switch {
			case level >= slog.LevelError:
				buf.WriteString(colorRed)
			case level >= slog.LevelWarn:
				buf.WriteString(colorYellow)
			case level >= slog.LevelInfo:
				buf.WriteString(colorGreen)
			default:
				buf.WriteString(colorBlue)
			}

			buf.WriteString(level.String())
			buf.WriteString(colorReset)
		} else {
			// Fallback for other Any types
			buf.WriteString(colorCyan)
			buf.WriteString(v.String())
			buf.WriteString(colorReset)
		}

	default:
		// Fallback for other types
		buf.WriteString(colorCyan)
		buf.WriteString(v.String())
		buf.WriteString(colorReset)
	}
}

// prettyJSONHandler implements a pretty-printed JSON handler for log messages.
type prettyJSONHandler struct {
	opts slog.HandlerOptions
	mu   *sync.Mutex
	w    io.Writer
}

func newPrettyJSONHandler(
	w io.Writer,
	opts *slog.HandlerOptions,
) *prettyJSONHandler {
	return &prettyJSONHandler{
		opts: *opts,
		mu:   &sync.Mutex{},
		w:    w,
	}
}

func (h *prettyJSONHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *prettyJSONHandler) Handle(_ context.Context, r slog.Record) error {
	buf := new(bytes.Buffer)

	buf.WriteString("{\n")

	// Add standard fields
	first := true
	if !r.Time.IsZero() {
		h.writeJSONField(
			buf,
			slog.TimeKey,
			r.Time.Format("2006-01-02T15:04:05Z07:00"),
			&first,
		)
	}

	h.writeJSONField(buf, slog.LevelKey, r.Level.String(), &first)

	// Write source if configured
	if h.opts.AddSource {
		if src := r.Source(); src != nil {
			// Format as file:line
			sourceStr := fmt.Sprintf("%s:%d", src.File, src.Line)
			h.writeJSONField(buf, slog.SourceKey, sourceStr, &first)
		}
	}

	h.writeJSONField(buf, slog.MessageKey, r.Message, &first)

	// Add custom attributes
	r.Attrs(func(a slog.Attr) bool {
		h.writeJSONAttr(buf, a, &first)

		return true
	})

	buf.WriteString("\n}")

	h.mu.Lock()
	defer h.mu.Unlock()

	_, err := h.w.Write(buf.Bytes())
	if err != nil {
		return err
	}

	_, err = h.w.Write([]byte("\n"))

	return err
}

func (h *prettyJSONHandler) WithAttrs([]slog.Attr) slog.Handler {
	return &prettyJSONHandler{
		opts: h.opts,
		mu:   h.mu,
		w:    h.w,
	}
}

func (h *prettyJSONHandler) WithGroup(string) slog.Handler {
	return &prettyJSONHandler{
		opts: h.opts,
		mu:   h.mu,
		w:    h.w,
	}
}

func (h *prettyJSONHandler) writeJSONField(
	buf *bytes.Buffer,
	key string,
	value any,
	first *bool,
) {
	if !*first {
		buf.WriteString(",\n")
	}

	*first = false

	buf.WriteString("  ")
	// Key in gray
	buf.WriteString(colorGray)
	buf.WriteString(key)
	buf.WriteString(colorReset)
	buf.WriteString(": ")

	// Write value based on type
	h.writeJSONValue(buf, value)
}

func (h *prettyJSONHandler) writeJSONAttr(
	buf *bytes.Buffer,
	a slog.Attr,
	first *bool,
) {
	if !*first {
		buf.WriteString(",\n")
	}

	*first = false

	buf.WriteString("  ")
	// Key in gray
	buf.WriteString(colorGray)
	buf.WriteString(a.Key)
	buf.WriteString(colorReset)
	buf.WriteString(": ")

	// Write value
	h.writeJSONValue(buf, a.Value.Any())
}

func (h *prettyJSONHandler) writeJSONValue(buf *bytes.Buffer, v any) {
	switch val := v.(type) {
	case string:
		// String without quotes, cyan color
		buf.WriteString(colorCyan)
		buf.WriteString(val)
		buf.WriteString(colorReset)

	case int, int8, int16, int32, int64:
		// Numbers in yellow
		buf.WriteString(colorYellow)
		fmt.Fprint(buf, val)
		buf.WriteString(colorReset)

	case uint, uint8, uint16, uint32, uint64:
		buf.WriteString(colorYellow)
		fmt.Fprint(buf, val)
		buf.WriteString(colorReset)

	case float32, float64:
		buf.WriteString(colorYellow)
		fmt.Fprint(buf, val)
		buf.WriteString(colorReset)

	case bool:
		if val {
			buf.WriteString(colorGreen)
			buf.WriteString("true")
		} else {
			buf.WriteString(colorRed)
			buf.WriteString("false")
		}

		buf.WriteString(colorReset)

	case nil:
		buf.WriteString(colorGray)
		buf.WriteString("null")
		buf.WriteString(colorReset)

	default:
		// For complex types, convert to string without quotes
		buf.WriteString(colorCyan)
		fmt.Fprint(buf, val)
		buf.WriteString(colorReset)
	}
}
