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

// Driver fans out log records to one or more handlers.
type Driver struct {
	mu          sync.Mutex
	handlers    atomic.Value
	sourceCache map[uintptr]callsite
}

// Handler delivers encoded log records to a single writer.
type Handler struct {
	writeMu  sync.Mutex
	configMu sync.Mutex
	config   atomic.Value
}

// New creates a Driver with one Handler per supplied HandlerOptions.
// It returns an error if any option set is invalid.
func New(options ...HandlerOptions) (*Driver, error) {
	driver := newDriver()
	for _, option := range options {
		if _, err := driver.AddHandler(option); err != nil {
			return nil, err
		}
	}
	return driver, nil
}

// Default returns the package-level driver.
// The default is a no-op until SetDefault is called or a handler is added.
func Default() *Driver {
	if driver, ok := defaultDriverStore.Load().(*Driver); ok && driver != nil {
		return driver
	}
	driver := newDriver()
	defaultDriverStore.Store(driver)
	return driver
}

// SetDefault replaces the package-level driver.
// Passing nil installs a fresh no-op driver.
func SetDefault(driver *Driver) {
	if driver == nil {
		driver = newDriver()
	}
	if driver.handlers.Load() == nil {
		driver.handlers.Store([]*Handler{})
	}
	defaultDriverStore.Store(driver)
}

// AddHandler creates a Handler from options and appends it to the driver.
func (driver *Driver) AddHandler(options HandlerOptions) (*Handler, error) {
	if driver == nil {
		return nil, fmt.Errorf("log: nil driver")
	}
	handler, err := newHandler(options)
	if err != nil {
		return nil, err
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	handlers := append([]*Handler(nil), driver.snapshotHandlers()...)
	handlers = append(handlers, handler)
	driver.handlers.Store(handlers)
	return handler, nil
}

// RemoveHandler removes the given handler from the driver.
// It reports whether the handler was found.
func (driver *Driver) RemoveHandler(handler *Handler) bool {
	if driver == nil || handler == nil {
		return false
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	handlers := driver.snapshotHandlers()
	for index, current := range handlers {
		if current != handler {
			continue
		}
		updated := make([]*Handler, 0, len(handlers)-1)
		updated = append(updated, handlers[:index]...)
		updated = append(updated, handlers[index+1:]...)
		driver.handlers.Store(updated)
		return true
	}
	return false
}

// Handlers returns an iterator over a snapshot of the driver's current handlers.
func (driver *Driver) Handlers() iter.Seq[*Handler] {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	handlers := driver.snapshotHandlers()
	return func(yield func(*Handler) bool) {
		for _, handler := range handlers {
			if !yield(handler) {
				break
			}
		}
	}
}

// Enabled reports whether the handler is currently active.
func (handler *Handler) Enabled() bool {
	config, ok := handler.snapshotConfig()
	if !ok {
		return false
	}
	return config.enabled
}

// Options returns the handler's current configuration and true.
// If the handler is uninitialized, returns an empty configuration and false.
func (handler *Handler) Options() (HandlerOptions, bool) {
	config, ok := handler.snapshotConfig()
	if !ok {
		return HandlerOptions{}, false
	}
	return HandlerOptions{
		Writer: config.writer,
		Format: config.format,
		Level:  config.level,
	}, true
}

// Writer returns the handler's current output destination and true.
// If the handler is uninitialized, returns nil and false.
func (handler *Handler) Writer() (io.Writer, bool) {
	config, ok := handler.snapshotConfig()
	if !ok {
		return nil, false
	}
	return config.writer, true
}

// Format returns the handler's current output encoding and true.
// If the handler is uninitialized, returns an invalid [Format] and false.
func (handler *Handler) Format() (Format, bool) {
	config, ok := handler.snapshotConfig()
	if !ok {
		var invalid Format
		return invalid, false
	}
	return config.format, true
}

// Level returns the handler's current maximum level and true.
// If the handler is uninitialized, returns an invalid [Level] and false.
func (handler *Handler) Level() (Level, bool) {
	config, ok := handler.snapshotConfig()
	if !ok {
		var invalid Level
		return invalid, false
	}
	return config.level, true
}

// Enable re-enables a previously disabled handler.
func (handler *Handler) Enable() error {
	return handler.updateConfig(func(config handlerConfig) (handlerConfig, error) {
		config.enabled = true
		return config, nil
	})
}

// Disable stops the handler from receiving log records without removing it.
func (handler *Handler) Disable() error {
	return handler.updateConfig(func(config handlerConfig) (handlerConfig, error) {
		config.enabled = false
		return config, nil
	})
}

// SetWriter replaces the handler's output writer.
// It returns an error if writer is nil.
func (handler *Handler) SetWriter(writer io.Writer) error {
	return handler.updateConfig(func(config handlerConfig) (handlerConfig, error) {
		if writer == nil {
			return config, fmt.Errorf("log: nil writer")
		}
		config.writer = writer
		config.target = detectOutputTarget(writer)
		return config, nil
	})
}

// SetFormat changes the handler's output encoding.
// It returns an error if format is invalid.
func (handler *Handler) SetFormat(format Format) error {
	return handler.updateConfig(func(config handlerConfig) (handlerConfig, error) {
		if !format.Valid() {
			return config, fmt.Errorf("log: invalid format %d", format)
		}
		config.format = format
		return config, nil
	})
}

// SetLevel changes the handler's maximum level.
// It returns an error if level is invalid.
func (handler *Handler) SetLevel(level Level) error {
	level = min(level, levelMax) // Clamp high - the user wants to see everything.
	return handler.updateConfig(func(config handlerConfig) (handlerConfig, error) {
		if !level.Valid() {
			return config, fmt.Errorf("log: invalid level %d", level)
		}
		config.level = level
		return config, nil
	})
}

// Log emits a record at level, joining parts with a single space.
func (driver *Driver) Log(level Level, attrs []slog.Attr, parts ...string) {
	driver.emit(level, attrs, joinParts(parts))
}

// Logf emits a record at level using fmt.Sprintf formatting.
func (driver *Driver) Logf(level Level, attrs []slog.Attr, format string, args ...any) {
	driver.emit(level, attrs, sprintfMessage(format, args))
}

// Error emits a record at LevelError, joining parts with a single space.
func (driver *Driver) Error(attrs []slog.Attr, parts ...string) {
	driver.emit(LevelError, attrs, joinParts(parts))
}

// Errorf emits a record at LevelError using fmt.Sprintf formatting.
func (driver *Driver) Errorf(attrs []slog.Attr, format string, args ...any) {
	driver.emit(LevelError, attrs, sprintfMessage(format, args))
}

// Warn emits a record at LevelWarn, joining parts with a single space.
func (driver *Driver) Warn(attrs []slog.Attr, parts ...string) {
	driver.emit(LevelWarn, attrs, joinParts(parts))
}

// Warnf emits a record at LevelWarn using fmt.Sprintf formatting.
func (driver *Driver) Warnf(attrs []slog.Attr, format string, args ...any) {
	driver.emit(LevelWarn, attrs, sprintfMessage(format, args))
}

// Info emits a record at LevelInfo, joining parts with a single space.
func (driver *Driver) Info(attrs []slog.Attr, parts ...string) {
	driver.emit(LevelInfo, attrs, joinParts(parts))
}

// Infof emits a record at LevelInfo using fmt.Sprintf formatting.
func (driver *Driver) Infof(attrs []slog.Attr, format string, args ...any) {
	driver.emit(LevelInfo, attrs, sprintfMessage(format, args))
}

// Debug emits a record at LevelDebug, joining parts with a single space.
// Terminal text output includes the call site for debug and trace records.
func (driver *Driver) Debug(attrs []slog.Attr, parts ...string) {
	driver.emit(LevelDebug, attrs, joinParts(parts))
}

// Debugf emits a record at LevelDebug using fmt.Sprintf formatting.
// Terminal text output includes the call site for debug and trace records.
func (driver *Driver) Debugf(attrs []slog.Attr, format string, args ...any) {
	driver.emit(LevelDebug, attrs, sprintfMessage(format, args))
}

// Trace emits a record at LevelTrace, joining parts with a single space.
// Terminal text output includes the call site for debug and trace records.
func (driver *Driver) Trace(attrs []slog.Attr, parts ...string) {
	driver.emit(LevelTrace, attrs, joinParts(parts))
}

// Tracef emits a record at LevelTrace using fmt.Sprintf formatting.
// Terminal text output includes the call site for debug and trace records.
func (driver *Driver) Tracef(attrs []slog.Attr, format string, args ...any) {
	driver.emit(LevelTrace, attrs, sprintfMessage(format, args))
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

func (driver *Driver) snapshotHandlers() []*Handler {
	if driver == nil {
		return nil
	}
	raw := driver.handlers.Load()
	if raw == nil {
		return nil
	}
	return raw.([]*Handler)
}

func (handler *Handler) snapshotConfig() (handlerConfig, bool) {
	if handler == nil {
		return handlerConfig{}, false
	}
	raw := handler.config.Load()
	if raw == nil {
		return handlerConfig{}, false
	}
	return raw.(handlerConfig), true
}

func (handler *Handler) updateConfig(update func(handlerConfig) (handlerConfig, error)) error {
	if handler == nil {
		return fmt.Errorf("log: nil handler")
	}
	handler.configMu.Lock()
	defer handler.configMu.Unlock()
	config, ok := handler.snapshotConfig()
	if !ok {
		return fmt.Errorf("log: uninitialized handler")
	}
	updated, err := update(config)
	if err != nil {
		return err
	}
	handler.config.Store(updated)
	return nil
}

func detectModuleRootPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}
