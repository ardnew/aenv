package log

import (
	"fmt"
	"io"
	"iter"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	longTimestampLayout  = "2006-01-02T15:04:05.000Z0700"
	shortTimestampLayout = "15:04:05.000"
)

var (
	timeNow            = time.Now
	moduleRoot         = detectModuleRootPath()
	defaultDriverStore atomic.Value
)

func init() {
	defaultDriverStore.Store(newDriver())
}

// Driver sends each record to its handlers.
type Driver struct {
	mu          sync.Mutex
	handlers    atomic.Value
	sourceCache map[uintptr]callsite
}

// Handler writes records to one writer.
type Handler struct {
	writeMu  sync.Mutex
	configMu sync.Mutex
	config   atomic.Value
}

// New returns a Driver with one handler per option. It errors on an invalid
// option.
func New(options ...HandlerOptions) (*Driver, error) {
	driver := newDriver()
	if err := driver.AddHandlers(options...); err != nil {
		return nil, err
	}
	return driver, nil
}

// Default returns the package driver. It is a no-op until a handler is added or
// SetDefault is called.
func Default() *Driver {
	if driver, ok := defaultDriverStore.Load().(*Driver); ok && driver != nil {
		return driver
	}
	driver := newDriver()
	defaultDriverStore.Store(driver)
	return driver
}

// SetDefault replaces the package driver. Nil installs a no-op driver.
func SetDefault(driver *Driver) {
	if driver == nil {
		driver = newDriver()
	}
	if driver.handlers.Load() == nil {
		driver.handlers.Store([]*Handler{})
	}
	defaultDriverStore.Store(driver)
}

// AddHandlers adds a [Handler] built from each of the given [HandlerOptions].
// It returns the first error from building a handler, if any, without adding
// remaining handlers.
func (d *Driver) AddHandlers(options ...HandlerOptions) error {
	if len(options) == 0 {
		return nil
	}
	if d == nil {
		return ErrNilDriver
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	// Copy the snapshot to avoid races with concurrent readers of the slice.
	snapshot := append([]*Handler(nil), d.snapshotHandlers()...)
	for _, option := range options {
		handler, err := newHandler(option)
		if err != nil {
			return err
		}
		snapshot = append(snapshot, handler)
	}
	d.handlers.Store(snapshot)
	return nil
}

// RemoveHandlers removes each given [Handler], or it removes all when called
// with no arguments, and returns whether any [Handler] was removed.
func (d *Driver) RemoveHandlers(handlers ...*Handler) bool {
	if d == nil {
		return false
	}
	if len(handlers) == 0 {
		return d.resetHandlers()
	}
	Trace(Attrs("requested", len(handlers)), "driver remove handlers")
	var found bool
	var updated []*Handler
	err := d.MapHandlers(
		func(h *Handler) bool {
			if h.isElementOf(handlers) {
				found = true
				return false
			}
			return true
		},
		func(h *Handler) error {
			updated = append(updated, h)
			return nil
		},
	)
	if err != nil {
		return false
	}
	if found {
		d.handlers.Store(updated)
		Trace(Attrs("remaining", len(updated)), "driver removed handlers")
		return true
	}
	Trace(Attrs("remaining", len(updated)), "driver remove handlers no-op")
	return false
}

// Handlers iterates a snapshot of the driver's handlers.
func (d *Driver) Handlers() iter.Seq[*Handler] {
	d.mu.Lock()
	defer d.mu.Unlock()
	handlers := d.snapshotHandlers()
	return func(yield func(*Handler) bool) {
		for _, handler := range handlers {
			if !yield(handler) {
				break
			}
		}
	}
}

// MapHandlers applies fn to each [Handler] predicated by want.
// It returns the first error returned by fn, if any, without evaluating fn on
// remaining handlers.
//
// It is safe to call [Handler] methods within fn, but calling [Driver] methods
// is undefined behavior and may cause a deadlock.
func (d *Driver) MapHandlers(
	want func(*Handler) bool,
	fn func(*Handler) error,
) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	handlers := d.snapshotHandlers()
	for _, handler := range handlers {
		if !want(handler) {
			continue
		}
		if err := fn(handler); err != nil {
			return err
		}
	}
	return nil
}

// AddHandlers calls [Driver.AddHandlers] on the package-level driver.
func AddHandlers(options ...HandlerOptions) error {
	return Default().AddHandlers(options...)
}

// RemoveHandlers calls [Driver.RemoveHandlers] on the package-level driver.
func RemoveHandlers(handlers ...*Handler) bool {
	return Default().RemoveHandlers(handlers...)
}

// Handlers calls [Driver.Handlers] on the package-level driver.
func Handlers() iter.Seq[*Handler] {
	return Default().Handlers()
}

// MapHandlers calls [Driver.MapHandlers] on the package-level driver.
func MapHandlers(want func(*Handler) bool, fn func(*Handler) error) error {
	return Default().MapHandlers(want, fn)
}

// IsTerminalHandler is a predicate matching enabled terminal writers.
func IsTerminalHandler(h *Handler) bool {
	return IsEnabledHandler(h) && h.IsTerminal()
}

// IsEnabledHandler is a predicate matching enabled [Handler]s.
func IsEnabledHandler(h *Handler) bool {
	return IsHandler(h) && h.Enabled()
}

// IsHandler is a predicate matching non-nil [Handler]s.
func IsHandler(h *Handler) bool {
	return h != nil
}

// Enabled reports whether the handler receives records.
func (h *Handler) Enabled() bool {
	config, ok := h.snapshotConfig()
	if !ok {
		return false
	}
	return config.enabled
}

// Options returns the handler's configuration. The second result is false if
// the handler is uninitialized.
func (h *Handler) Options() (HandlerOptions, bool) {
	config, ok := h.snapshotConfig()
	if !ok {
		return HandlerOptions{}, false
	}
	return HandlerOptions{
		Writer: config.writer,
		Format: config.format,
		Level:  config.level,
	}, true
}

// Writer returns the handler's output. The second result is false if the
// handler is uninitialized.
func (h *Handler) Writer() (io.Writer, bool) {
	config, ok := h.snapshotConfig()
	if !ok {
		return nil, false
	}
	return config.writer, true
}

// IsTerminal returns whether the handler writes to a terminal.
// It also returns false if the handler is uninitialized.
func (h *Handler) IsTerminal() bool {
	config, ok := h.snapshotConfig()
	if !ok {
		return false
	}
	return config.target == targetTerminal
}

// Format returns the handler's output encoding. The second result is false if
// the handler is uninitialized.
func (h *Handler) Format() (Format, bool) {
	config, ok := h.snapshotConfig()
	if !ok {
		var invalid Format
		return invalid, false
	}
	return config.format, true
}

// Level returns the handler's highest forwarded level. The second result is
// false if the handler is uninitialized.
func (h *Handler) Level() (Level, bool) {
	config, ok := h.snapshotConfig()
	if !ok {
		var invalid Level
		return invalid, false
	}
	return config.level, true
}

// Enable resumes a disabled handler.
func (h *Handler) Enable() error {
	return h.updateConfig(func(config handlerConfig) (handlerConfig, error) {
		config.enabled = true
		return config, nil
	})
}

// Disable stops the handler from receiving records.
func (h *Handler) Disable() error {
	return h.updateConfig(func(config handlerConfig) (handlerConfig, error) {
		config.enabled = false
		return config, nil
	})
}

// SetWriter replaces the output. It errors on a nil writer.
func (h *Handler) SetWriter(writer io.Writer) error {
	return h.updateConfig(func(config handlerConfig) (handlerConfig, error) {
		if writer == nil {
			return config, ErrNilWriter
		}
		config.writer = writer
		config.target = detectOutputTarget(writer)
		return config, nil
	})
}

// SetFormat replaces the output encoding. It errors on an invalid format.
func (h *Handler) SetFormat(format Format) error {
	return h.updateConfig(func(config handlerConfig) (handlerConfig, error) {
		if !format.Valid() {
			return config, errf(ErrInvalidFormat, "%d", format)
		}
		config.format = format
		return config, nil
	})
}

// SetLevel sets the highest level forwarded. It errors on an invalid level.
func (h *Handler) SetLevel(level Level) error {
	level = min(level, levelMax) // Clamp high - the user wants to see everything.
	return h.updateConfig(func(config handlerConfig) (handlerConfig, error) {
		if !level.Valid() {
			return config, errf(ErrInvalidLevel, "%d", level)
		}
		config.level = level
		return config, nil
	})
}

// Log emits a record at level. Parts join with a space.
func (d *Driver) Log(level Level, attrs []slog.Attr, parts ...string) {
	d.emit(level, attrs, joinParts(parts))
}

// Logf emits a record at level, formatted by fmt.Sprintf.
func (d *Driver) Logf(level Level, attrs []slog.Attr, format string, args ...any) {
	d.emit(level, attrs, sprintfMessage(format, args))
}

// Error emits a record at LevelError. Parts join with a space.
func (d *Driver) Error(attrs []slog.Attr, parts ...string) {
	d.emit(LevelError, attrs, joinParts(parts))
}

// Errorf emits a record at LevelError, formatted by fmt.Sprintf.
func (d *Driver) Errorf(attrs []slog.Attr, format string, args ...any) {
	d.emit(LevelError, attrs, sprintfMessage(format, args))
}

// Warn emits a record at LevelWarn. Parts join with a space.
func (d *Driver) Warn(attrs []slog.Attr, parts ...string) {
	d.emit(LevelWarn, attrs, joinParts(parts))
}

// Warnf emits a record at LevelWarn, formatted by fmt.Sprintf.
func (d *Driver) Warnf(attrs []slog.Attr, format string, args ...any) {
	d.emit(LevelWarn, attrs, sprintfMessage(format, args))
}

// Info emits a record at LevelInfo. Parts join with a space.
func (d *Driver) Info(attrs []slog.Attr, parts ...string) {
	d.emit(LevelInfo, attrs, joinParts(parts))
}

// Infof emits a record at LevelInfo, formatted by fmt.Sprintf.
func (d *Driver) Infof(attrs []slog.Attr, format string, args ...any) {
	d.emit(LevelInfo, attrs, sprintfMessage(format, args))
}

// Debug emits a record at LevelDebug. Parts join with a space.
// Terminal text adds the call site.
func (d *Driver) Debug(attrs []slog.Attr, parts ...string) {
	d.emit(LevelDebug, attrs, joinParts(parts))
}

// Debugf emits a record at LevelDebug, formatted by fmt.Sprintf.
// Terminal text adds the call site.
func (d *Driver) Debugf(attrs []slog.Attr, format string, args ...any) {
	d.emit(LevelDebug, attrs, sprintfMessage(format, args))
}

// Trace emits a record at LevelTrace. Parts join with a space.
// Terminal text adds the call site.
func (d *Driver) Trace(attrs []slog.Attr, parts ...string) {
	d.emit(LevelTrace, attrs, joinParts(parts))
}

// Tracef emits a record at LevelTrace, formatted by fmt.Sprintf.
// Terminal text adds the call site.
func (d *Driver) Tracef(attrs []slog.Attr, format string, args ...any) {
	d.emit(LevelTrace, attrs, sprintfMessage(format, args))
}

// Log calls Log on the package-level driver.
func Log(level Level, attrs []slog.Attr, parts ...string) {
	Default().emit(level, attrs, joinParts(parts))
}

// Logf calls Logf on the package-level driver.
func Logf(level Level, attrs []slog.Attr, format string, args ...any) {
	Default().emit(level, attrs, sprintfMessage(format, args))
}

// Error calls Error on the package-level driver.
func Error(attrs []slog.Attr, parts ...string) {
	Default().emit(LevelError, attrs, joinParts(parts))
}

// Errorf calls Errorf on the package-level driver.
func Errorf(attrs []slog.Attr, format string, args ...any) {
	Default().emit(LevelError, attrs, sprintfMessage(format, args))
}

// Warn calls Warn on the package-level driver.
func Warn(attrs []slog.Attr, parts ...string) {
	Default().emit(LevelWarn, attrs, joinParts(parts))
}

// Warnf calls Warnf on the package-level driver.
func Warnf(attrs []slog.Attr, format string, args ...any) {
	Default().emit(LevelWarn, attrs, sprintfMessage(format, args))
}

// Info calls Info on the package-level driver.
func Info(attrs []slog.Attr, parts ...string) {
	Default().emit(LevelInfo, attrs, joinParts(parts))
}

// Infof calls Infof on the package-level driver.
func Infof(attrs []slog.Attr, format string, args ...any) {
	Default().emit(LevelInfo, attrs, sprintfMessage(format, args))
}

// Debug calls Debug on the package-level driver.
func Debug(attrs []slog.Attr, parts ...string) {
	Default().emit(LevelDebug, attrs, joinParts(parts))
}

// Debugf calls Debugf on the package-level driver.
func Debugf(attrs []slog.Attr, format string, args ...any) {
	Default().emit(LevelDebug, attrs, sprintfMessage(format, args))
}

// Trace calls Trace on the package-level driver.
func Trace(attrs []slog.Attr, parts ...string) {
	Default().emit(LevelTrace, attrs, joinParts(parts))
}

// Tracef calls Tracef on the package-level driver.
func Tracef(attrs []slog.Attr, format string, args ...any) {
	Default().emit(LevelTrace, attrs, sprintfMessage(format, args))
}

// Attrs constructs a slice of slog.Attr from alternating keys and values.
//
// Keys that cannot be asserted as strings are replaced with a generic
// identifier "{!key[%d]}" containing its argument index.
//
// Values are constructed with [slog.Any] for deriving the type, which provides
// maximum support for types handled natively by [slog.Handler]s.
//
// If an odd number of arguments is provided, the final key is silently ignored.
// Pairs with nil values are also sildently ignored.
func Attrs(keyVals ...any) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(keyVals)/2)
	for i := 0; i < len(keyVals)-1; i += 2 {
		key, ok := keyVals[i].(string)
		if !ok {
			key = fmt.Sprintf("{!key[%d]}", i/2)
		}
		if keyVals[i+1] != nil {
			attrs = append(attrs, slog.Any(key, keyVals[i+1]))
		}
	}
	return attrs
}

// Group returns a grouped [slog.Attr] wrapped in a slice.
// The group is constructed with the given key and the slice returned from
// [Attrs] with the remaining arguments.
func Group(key string, keyVals ...any) []slog.Attr {
	return []slog.Attr{slog.GroupAttrs(key, Attrs(keyVals...)...)}
}

// joinParts returns a message builder that joins parts with a single space.
// The builder is invoked lazily by emit only when a handler will receive the
// record, avoiding the join cost for silenced records.
func joinParts(parts []string) func() string {
	return func() string { return strings.Join(parts, " ") }
}

// sprintfMessage returns a message builder that formats args per format.
// The builder is invoked lazily by emit only when a handler will receive the
// record, avoiding the formatting cost for silenced records.
func sprintfMessage(format string, args []any) func() string {
	return func() string { return fmt.Sprintf(format, args...) }
}

func newDriver() *Driver {
	driver := &Driver{sourceCache: make(map[uintptr]callsite)}
	driver.handlers.Store([]*Handler{})
	return driver
}

func newHandler(options HandlerOptions) (*Handler, error) {
	config, err := newHandlerConfig(options)
	if err != nil {
		return nil, err
	}
	handler := &Handler{}
	handler.config.Store(config)
	return handler, nil
}

func (d *Driver) snapshotHandlers() []*Handler {
	if d == nil {
		return nil
	}
	raw := d.handlers.Load()
	if raw == nil {
		return nil
	}
	return raw.([]*Handler)
}

func (d *Driver) resetHandlers() bool {
	if d == nil {
		return false
	}
	count := len(d.snapshotHandlers())
	d.handlers.Store([]*Handler{})
	Trace(Attrs("removed", count), "driver reset handlers")
	return count > 0
}

func (h *Handler) snapshotConfig() (handlerConfig, bool) {
	if h == nil {
		return handlerConfig{}, false
	}
	raw := h.config.Load()
	if raw == nil {
		return handlerConfig{}, false
	}
	return raw.(handlerConfig), true
}

func (h *Handler) updateConfig(update func(handlerConfig) (handlerConfig, error)) error {
	if h == nil {
		return ErrNilHandler
	}
	h.configMu.Lock()
	defer h.configMu.Unlock()
	config, ok := h.snapshotConfig()
	if !ok {
		return ErrZeroHandler
	}
	updated, err := update(config)
	if err != nil {
		return err
	}
	h.config.Store(updated)
	return nil
}

func (h *Handler) isElementOf(handlers []*Handler) bool {
	for _, handler := range handlers {
		if h == handler {
			return true
		}
	}
	return false
}

func detectModuleRootPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}
