package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	userAttrsKey         = "attr"
	textMessageDelimiter = ":: "
)

// reprVariant is a pair of representations of the same value:
// a detailed, long format and a concise, short format.
type reprVariant[T any] struct{ long, short T }

type emitNeeds struct {
	timestamp reprVariant[bool]
	whence    reprVariant[bool]
}

type callsite struct {
	scope    string
	position reprVariant[string]
}

type eventRecord struct {
	level     Level
	timestamp reprVariant[string]
	whence    callsite
	message   string
	attrs     []slog.Attr
}

func (e eventRecord) resolveVariants(driver *Driver, needs emitNeeds) eventRecord {
	now := timeNow()
	if needs.timestamp.long {
		e.timestamp.long = now.Format(longTimestampLayout)
	}
	if needs.timestamp.short {
		e.timestamp.short = now.Format(shortTimestampLayout)
	}
	if needs.whence.long || needs.whence.short {
		pc, file, line, ok := nextCallerFrame()
		if ok {
			source := driver.lookupSource(pc, file, line)
			e.whence.scope = source.scope
			if needs.whence.long {
				e.whence.position.long = source.position.long
			}
			if needs.whence.short {
				e.whence.position.short = source.position.short
			}
		}
	}
	return e
}

func (driver *Driver) emit(level Level, attrs []slog.Attr, buildMessage func() string) {
	if driver == nil || !level.Valid() {
		return
	}
	candidates := driver.selectHandlers(level)
	if len(candidates) == 0 {
		return
	}

	record := eventRecord{
		level:   level,
		message: buildMessage(),
		attrs:   normalizeEventAttrs(attrs),
	}.resolveVariants(
		driver,
		gatherEmitNeeds(candidates),
	)

	for _, candidate := range candidates {
		candidate.handler.write(candidate.config, record)
	}
}

func nextCallerFrame() (uintptr, string, int, bool) {
	var pcs [16]uintptr
	count := runtime.Callers(3, pcs[:])
	frames := runtime.CallersFrames(pcs[:count])
	for {
		frame, more := frames.Next()
		if !isInternalLogFrame(frame.File) {
			return frame.PC, frame.File, frame.Line, true
		}
		if !more {
			break
		}
	}
	return 0, "", 0, false
}

func isInternalLogFrame(path string) bool {
	if moduleRoot == "" {
		base := filepath.Base(path)
		return base == "driver.go" || base == "encode.go"
	}
	rel, err := filepath.Rel(moduleRoot, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	return rel == "log/driver.go" || rel == "log/encode.go"
}

func (driver *Driver) selectHandlers(level Level) []handlerCandidate {
	handlers := driver.snapshotHandlers()
	selected := make([]handlerCandidate, 0, len(handlers))
	for _, handler := range handlers {
		config, ok := handler.snapshotConfig()
		if !ok || !config.enabled || !config.level.Allows(level) {
			continue
		}
		selected = append(selected, handlerCandidate{handler: handler, config: config})
	}
	return selected
}

func gatherEmitNeeds(candidates []handlerCandidate) emitNeeds {
	var needs emitNeeds
	for _, candidate := range candidates {
		if candidate.config.target == targetFile {
			needs.timestamp.long = true
			needs.whence.long = true
			continue
		}
		needs.timestamp.short = true
		if candidate.config.format == FormatJSON {
			needs.whence.long = true
			continue
		}
		if candidate.config.level == LevelDebug || candidate.config.level == LevelTrace {
			needs.whence.short = true
		}
	}
	return needs
}

func (driver *Driver) lookupSource(pc uintptr, file string, line int) callsite {
	driver.mu.Lock()
	if cached, ok := driver.sourceCache[pc]; ok {
		driver.mu.Unlock()
		return cached
	}
	driver.mu.Unlock()

	relativePath := filepath.Base(file)
	if moduleRoot != "" {
		rel, err := filepath.Rel(moduleRoot, file)
		if err == nil {
			relativePath = filepath.ToSlash(rel)
		}
	}
	funcName := "unknown"
	if fn := runtime.FuncForPC(pc); fn != nil {
		funcName = trimRuntimeFuncName(fn.Name())
	}
	source := callsite{
		scope: funcName,
		position: reprVariant[string]{
			long:  relativePath + ":" + strconv.Itoa(line),
			short: filepath.Base(file) + ":" + strconv.Itoa(line),
		},
	}

	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.sourceCache == nil {
		driver.sourceCache = make(map[uintptr]callsite)
	} else if cached, ok := driver.sourceCache[pc]; ok {
		return cached
	}
	driver.sourceCache[pc] = source
	return source
}

func trimRuntimeFuncName(name string) string {
	if index := strings.LastIndex(name, "/"); index >= 0 {
		name = name[index+1:]
	}
	return name
}

func normalizeEventAttrs(attrs []slog.Attr) []slog.Attr {
	normalized := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		normalizedAttr, ok := normalizeEventAttr(attr)
		if ok {
			normalized = append(normalized, normalizedAttr)
		}
	}
	return normalized
}

func normalizeEventAttr(attr slog.Attr) (slog.Attr, bool) {
	attr.Value = attr.Value.Resolve()
	if attr.Value.Kind() != slog.KindGroup {
		if attr.Key == "" {
			return slog.Attr{}, false
		}
		return attr, true
	}
	children := make([]slog.Attr, 0, len(attr.Value.Group()))
	for _, child := range attr.Value.Group() {
		normalizedChild, ok := normalizeEventAttr(child)
		if ok {
			children = append(children, normalizedChild)
		}
	}
	if attr.Key == "" && len(children) == 0 {
		return slog.Attr{}, false
	}
	attr.Value = slog.GroupValue(children...)
	return attr, true
}

func (handler *Handler) write(config handlerConfig, record eventRecord) {
	encoded := encodeEvent(config, record)
	if len(encoded) == 0 {
		return
	}
	handler.writeMu.Lock()
	defer handler.writeMu.Unlock()
	_, _ = config.writer.Write(encoded)
}

func encodeEvent(config handlerConfig, record eventRecord) []byte {
	if config.format == FormatJSON {
		return encodeJSONEvent(config.target, record)
	}
	return encodeTextEvent(config.target, config.level, record)
}

func encodeTextEvent(target outputTarget, handlerLevel Level, record eventRecord) []byte {
	var buf bytes.Buffer
	if target == targetFile {
		buf.WriteString("time=")
		buf.WriteString(record.timestamp.long)
		buf.WriteString(" level=")
		buf.WriteString(record.level.String())
		appendTextSourceScope(&buf, record.whence.position.long, record.whence.scope)
		appendTextAttrs(&buf, userAttrsKey, record.attrs)
		buf.WriteString(" message=")
		buf.WriteString(record.message)
		buf.WriteByte('\n')
		return buf.Bytes()
	}

	buf.WriteString(record.timestamp.short)
	buf.WriteByte(' ')
	buf.WriteString(record.level.Symbol())

	var optLen int
	if handlerLevel == LevelDebug || handlerLevel == LevelTrace {
		optLen += appendTextSourceScope(&buf, record.whence.position.short, record.whence.scope)
	}
	optLen += appendTextAttrs(&buf, userAttrsKey, record.attrs)
	if record.message != "" {
		buf.WriteByte(' ')
		if optLen > 0 {
			buf.WriteString(textMessageDelimiter)
		}
		buf.WriteString(record.message)
	}
	buf.WriteByte('\n')
	return buf.Bytes()
}

func appendTextSourceScope(buf *bytes.Buffer, source string, scope string) int {
	n := buf.Len() // count the number of bytes already in the buffer
	if source != "" {
		buf.WriteString(" source=")
		buf.WriteString(quoteTextValue(source))
	}
	if scope != "" {
		buf.WriteString(" scope=")
		buf.WriteString(quoteTextValue(scope))
	}
	return buf.Len() - n // count the number of bytes written
}

func appendTextAttrs(buf *bytes.Buffer, prefix string, attrs []slog.Attr) int {
	n := buf.Len() // count the number of bytes already in the buffer
	appendTextAttrValues(buf, prefix, attrs)
	return buf.Len() - n // count the number of bytes written
}

func appendTextAttrValues(buf *bytes.Buffer, prefix string, attrs []slog.Attr) {
	for _, attr := range attrs {
		appendTextAttr(buf, prefix, attr)
	}
}

func appendTextAttr(buf *bytes.Buffer, prefix string, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()
	name := attr.Key
	if prefix != "" {
		if name == "" {
			name = prefix
		} else {
			name = prefix + "." + name
		}
	}
	if attr.Value.Kind() == slog.KindGroup {
		nextPrefix := prefix
		if attr.Key != "" {
			nextPrefix = name
		}
		appendTextAttrValues(buf, nextPrefix, attr.Value.Group())
		return
	}
	if name == "" {
		return
	}
	buf.WriteByte(' ')
	buf.WriteString(name)
	buf.WriteByte('=')
	buf.WriteString(formatTextValue(attr.Value))
}

func formatTextValue(value slog.Value) string {
	value = value.Resolve()
	switch value.Kind() {
	case slog.KindString:
		return quoteTextValue(value.String())
	case slog.KindBool:
		return strconv.FormatBool(value.Bool())
	case slog.KindInt64:
		return strconv.FormatInt(value.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(value.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(value.Float64(), 'g', -1, 64)
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindTime:
		return value.Time().Format(time.RFC3339Nano)
	case slog.KindAny:
		return quoteTextValue(fmt.Sprint(value.Any()))
	default:
		return quoteTextValue(value.String())
	}
}

func quoteTextValue(text string) string {
	if text == "" || strings.ContainsAny(text, " \t\r\n\"=") {
		return strconv.Quote(text)
	}
	return text
}

func encodeJSONEvent(target outputTarget, record eventRecord) []byte {
	var buf bytes.Buffer
	first := true
	timestamp := record.timestamp.long
	if target == targetTerminal {
		timestamp = record.timestamp.short
	}
	buf.WriteByte('{')
	appendJSONField(&buf, &first, "time", timestamp)
	appendJSONField(&buf, &first, "level", record.level.String())
	if record.whence.position.long != "" {
		appendJSONField(&buf, &first, "source", record.whence.position.long)
	}
	if record.whence.scope != "" {
		appendJSONField(&buf, &first, "scope", record.whence.scope)
	}
	if len(record.attrs) > 0 {
		appendJSONAttrGroupField(&buf, &first, userAttrsKey, record.attrs)
	}
	appendJSONField(&buf, &first, "message", record.message)
	buf.WriteString("}\n")
	return buf.Bytes()
}

func appendJSONAttrGroupField(buf *bytes.Buffer, first *bool, key string, attrs []slog.Attr) {
	if len(attrs) == 0 {
		return
	}
	if !*first {
		buf.WriteByte(',')
	}
	*first = false
	appendJSONKey(buf, key)
	buf.WriteByte(':')
	appendJSONAttrObject(buf, attrs)
}

func appendJSONAttrObject(buf *bytes.Buffer, attrs []slog.Attr) {
	buf.WriteByte('{')
	first := true
	appendJSONAttrMembers(buf, &first, attrs)
	buf.WriteByte('}')
}

func appendJSONAttrMembers(buf *bytes.Buffer, first *bool, attrs []slog.Attr) {
	for _, attr := range attrs {
		appendJSONAttr(buf, first, attr)
	}
}

func appendJSONAttr(buf *bytes.Buffer, first *bool, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()
	if attr.Value.Kind() == slog.KindGroup {
		if attr.Key == "" {
			appendJSONAttrMembers(buf, first, attr.Value.Group())
			return
		}
		if !*first {
			buf.WriteByte(',')
		}
		*first = false
		appendJSONKey(buf, attr.Key)
		buf.WriteByte(':')
		appendJSONAttrObject(buf, attr.Value.Group())
		return
	}
	if attr.Key == "" {
		return
	}
	if !*first {
		buf.WriteByte(',')
	}
	*first = false
	appendJSONKey(buf, attr.Key)
	buf.WriteByte(':')
	appendJSONValue(buf, attr.Value)
}

func appendJSONField(buf *bytes.Buffer, first *bool, key, value string) {
	if !*first {
		buf.WriteByte(',')
	}
	*first = false
	appendJSONKey(buf, key)
	buf.WriteByte(':')
	appendJSONString(buf, value)
}

// appendJSONKey writes a JSON object key using Go-syntax quoting, reusing the
// buffer's spare capacity to avoid a per-key allocation.
func appendJSONKey(buf *bytes.Buffer, key string) {
	buf.Write(strconv.AppendQuote(buf.AvailableBuffer(), key))
}

// appendJSONValue writes value as a JSON token, encoding primitives directly to
// avoid per-value reflection and allocation. Floats and arbitrary values fall
// back to encoding/json to preserve its exact formatting.
func appendJSONValue(buf *bytes.Buffer, value slog.Value) {
	value = value.Resolve()
	switch value.Kind() {
	case slog.KindString:
		appendJSONString(buf, value.String())
	case slog.KindBool:
		buf.Write(strconv.AppendBool(buf.AvailableBuffer(), value.Bool()))
	case slog.KindInt64:
		buf.Write(strconv.AppendInt(buf.AvailableBuffer(), value.Int64(), 10))
	case slog.KindUint64:
		buf.Write(strconv.AppendUint(buf.AvailableBuffer(), value.Uint64(), 10))
	case slog.KindDuration:
		appendJSONString(buf, value.Duration().String())
	case slog.KindTime:
		appendJSONString(buf, value.Time().Format(time.RFC3339Nano))
	case slog.KindFloat64, slog.KindAny:
		buf.Write(marshalJSONValue(jsonFieldValue(value)))
	default:
		appendJSONString(buf, value.String())
	}
}

// appendJSONString writes s as a quoted JSON string, matching encoding/json's
// default (HTML-escaping) encoder byte-for-byte without allocating.
func appendJSONString(buf *bytes.Buffer, s string) {
	const hex = "0123456789abcdef"
	buf.WriteByte('"')
	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if jsonSafe(b) {
				i++
				continue
			}
			if start < i {
				buf.WriteString(s[start:i])
			}
			switch b {
			case '\\', '"':
				buf.WriteByte('\\')
				buf.WriteByte(b)
			case '\n':
				buf.WriteString(`\n`)
			case '\r':
				buf.WriteString(`\r`)
			case '\t':
				buf.WriteString(`\t`)
			default:
				buf.WriteString(`\u00`)
				buf.WriteByte(hex[b>>4])
				buf.WriteByte(hex[b&0xF])
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			if start < i {
				buf.WriteString(s[start:i])
			}
			buf.WriteString(`\ufffd`)
			i += size
			start = i
			continue
		}
		if c == '\u2028' || c == '\u2029' {
			if start < i {
				buf.WriteString(s[start:i])
			}
			buf.WriteString(`\u202`)
			buf.WriteByte(hex[c&0xF])
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		buf.WriteString(s[start:])
	}
	buf.WriteByte('"')
}

// jsonSafe reports whether the ASCII byte can be emitted in a JSON string
// without escaping under encoding/json's default HTML-escaping policy.
func jsonSafe(b byte) bool {
	return b >= 0x20 && b != '"' && b != '\\' && b != '<' && b != '>' && b != '&'
}

func marshalJSONValue(value any) []byte {
	data, err := json.Marshal(value)
	if err == nil {
		return data
	}
	fallback, _ := json.Marshal(fmt.Sprint(value))
	return fallback
}

func jsonFieldValue(value slog.Value) any {
	value = value.Resolve()
	switch value.Kind() {
	case slog.KindString:
		return value.String()
	case slog.KindBool:
		return value.Bool()
	case slog.KindInt64:
		return value.Int64()
	case slog.KindUint64:
		return value.Uint64()
	case slog.KindFloat64:
		return value.Float64()
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindTime:
		return value.Time().Format(time.RFC3339Nano)
	case slog.KindAny:
		return value.Any()
	default:
		return value.String()
	}
}
