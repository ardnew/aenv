// Package log provides synchronous multi-handler logging for this module.
//
// Handlers are configured by writer, format, and maximum level. The package
// derives output behavior — file versus terminal layout and source formatting
// — from those inputs.
//
// The package-level driver is a no-op until callers install handlers on it or
// replace it with SetDefault.
//
// Logging methods accept attrs as []slog.Attr followed by variadic message
// parts or a printf-style format string:
//
//	Log(level Level, attrs []slog.Attr, parts ...string)
//	Logf(level Level, attrs []slog.Attr, format string, args ...any)
//
// Each level also has a dedicated pair — Error/Errorf, Warn/Warnf, Info/Infof,
// Debug/Debugf, Trace/Tracef — that omits the explicit Level argument:
//
//	log.Info([]slog.Attr{slog.String("scope", "cli")}, "starting", "up")
//	log.Infof(nil, "loaded %d files", count)
//
// Create a driver with multiple handlers:
//
//	driver, err := log.New(
//		log.HandlerOptions{Writer: os.Stdout, Format: log.FormatText, Level: log.LevelWarn},
//		log.HandlerOptions{Writer: file, Format: log.FormatJSON, Level: log.LevelInfo},
//	)
//	if err != nil {
//		return err
//	}
//
// Install a configured driver as the package default:
//
//	log.SetDefault(driver)
//	log.Info([]slog.Attr{slog.String("component", "sync")}, "loaded", "config")
//
// Reconfigure a handler at runtime:
//
//	handler, err := driver.AddHandler(
//		log.HandlerOptions{
//			Writer: os.Stdout,
//			Format: log.FormatText,
//			Level:  log.LevelWarn,
//		},
//	)
//	if err != nil {
//		return err
//	}
//	if err := handler.SetLevel(log.LevelInfo); err != nil {
//		return err
//	}
//	driver.Info(nil, "visible", "now")
//
// Inspect handler configuration at runtime:
//
//	for handler := range driver.Handlers() {
//		level, ok := handler.Level()
//		if !ok {
//			continue
//		}
//		fmt.Printf("level=%s enabled=%v\n", level, handler.Enabled())
//	}
//
// Each record carries these built-in fields: time, level, source (path:line),
// scope (package.function), and message. source and scope identify the original
// log call site, not the handler. User attributes are collected under the attr
// namespace: as a nested attr object in JSON, and as attr.key=value pairs in
// text. The attr namespace is omitted when a record has no user attributes.
// Both formats produce one record per line.
//
// Handlers at Debug or Trace include the call site in all terminal text output;
// handlers at other levels omit it.
//
// File writers use a long ISO 8601 timestamp and the full source path relative
// to the module root. Terminal writers use a short HH:MM:SS.mmm timestamp and,
// for text output, the base file name only.
//
// Output format — file writers:
//
//	text: time=TIMESTAMP level=LEVEL source=PATH:LINE scope=PKG.FUNC [attr.KEY=VALUE...] message=TEXT
//	JSON: {"time":"TIMESTAMP","level":"LEVEL","source":"PATH:LINE","scope":"PKG.FUNC","attr":{...},"message":"TEXT"}
//
// The JSON attr field is omitted when there are no user attributes.
//
// Output format — terminal writers:
//
// Text omits key names for built-in fields. Fields are space-separated; the
// message is omitted when empty, follows a single space when no user attributes
// are present, and otherwise uses the " :: " separator:
//
//	Error/Warn/Info (no attrs):   TIMESTAMP SYMBOL MESSAGE
//	Error/Warn/Info (with attrs): TIMESTAMP SYMBOL attr.KEY=VALUE... :: MESSAGE
//	Debug or Trace (no attrs):    TIMESTAMP SYMBOL source=FILE:LINE scope=PKG.FUNC :: MESSAGE
//	Debug or Trace (with attrs):  TIMESTAMP SYMBOL source=FILE:LINE scope=PKG.FUNC attr.KEY=VALUE... :: MESSAGE
//
// SYMBOL is the level symbol (see Level.Symbol). JSON at a terminal uses the
// long source path and the same schema as file JSON, but with the short
// timestamp. Writers that cannot be confirmed as terminals use the file layout.
package log
