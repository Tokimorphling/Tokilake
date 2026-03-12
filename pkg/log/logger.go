package log

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	errorKey  = "LOG_ERROR"
	callerKey = "caller"
	fileKey   = "file"
	lineKey   = "line"
)

const (
	legacyLevelCrit = iota
	legacyLevelError
	legacyLevelWarn
	legacyLevelInfo
	legacyLevelDebug
	legacyLevelTrace
)

const (
	levelMaxVerbosity slog.Level = math.MinInt
	LevelTrace        slog.Level = -8
	LevelDebug                   = slog.LevelDebug
	LevelInfo                    = slog.LevelInfo
	LevelWarn                    = slog.LevelWarn
	LevelError                   = slog.LevelError
	LevelCrit         slog.Level = 12

	LvlTrace = LevelTrace
	LvlInfo  = LevelInfo
	LvlDebug = LevelDebug
)

func FromLegacyLevel(lvl int) slog.Level {
	switch lvl {
	case legacyLevelCrit:
		return LevelCrit
	case legacyLevelError:
		return LevelError
	case legacyLevelWarn:
		return LevelWarn
	case legacyLevelInfo:
		return LevelInfo
	case legacyLevelDebug:
		return LevelDebug
	case legacyLevelTrace:
		return LevelTrace
	default:
		if lvl > legacyLevelTrace {
			return LevelTrace
		}
		return LevelCrit
	}
}

func LevelAlignedString(l slog.Level) string {
	switch l {
	case LevelTrace:
		return "TRACE"
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return "INFO "
	case slog.LevelWarn:
		return "WARN "
	case slog.LevelError:
		return "ERROR"
	case LevelCrit:
		return "CRIT "
	default:
		return "unknown level"
	}
}

func LevelString(l slog.Level) string {
	switch l {
	case LevelTrace:
		return "trace"
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	case LevelCrit:
		return "crit"
	default:
		return "unknown"
	}
}

func StirngLevel(lvl string) slog.Level {
	switch lvl {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "crit":
		return LevelCrit
	case "trace":
		return LevelTrace
	default:
		return slog.LevelInfo
	}
}

type Logger interface {
	With(ctx ...interface{}) Logger
	New(ctx ...interface{}) Logger
	Log(level slog.Level, msg string, ctx ...interface{})
	Trace(msg string, ctx ...interface{})
	Debug(msg string, ctx ...interface{})
	Info(msg string, ctx ...interface{})
	Warn(msg string, ctx ...interface{})
	Error(msg string, ctx ...interface{})
	Crit(msg string, ctx ...interface{})
	Write(level slog.Level, msg string, attrs ...any)
	Enabled(ctx context.Context, level slog.Level) bool
	Handler() slog.Handler
}

type logger struct {
	inner *slog.Logger
}

func NewLogger(h slog.Handler) Logger {
	return &logger{inner: slog.New(h)}
}

func (l *logger) Handler() slog.Handler {
	return l.inner.Handler()
}

func (l *logger) Write(level slog.Level, msg string, attrs ...any) {
	if !l.inner.Enabled(context.Background(), level) {
		return
	}

	pc, callerAttrs := captureCaller()
	userAttrs, warnAttr := normalizeAttrs(attrs)
	record := slog.NewRecord(time.Now().UTC(), level, msg, pc)
	if len(callerAttrs) > 0 {
		record.AddAttrs(callerAttrs...)
	}
	if len(userAttrs) > 0 {
		record.AddAttrs(userAttrs...)
	}
	if warnAttr != nil {
		record.AddAttrs(*warnAttr)
	}
	_ = l.inner.Handler().Handle(context.Background(), record)
}

func (l *logger) Log(level slog.Level, msg string, attrs ...any) {
	l.Write(level, msg, attrs...)
}

func (l *logger) With(ctx ...interface{}) Logger {
	return &logger{inner: l.inner.With(ctx...)}
}

func (l *logger) New(ctx ...interface{}) Logger {
	return l.With(ctx...)
}

func (l *logger) Enabled(ctx context.Context, level slog.Level) bool {
	return l.inner.Enabled(ctx, level)
}

func (l *logger) Trace(msg string, ctx ...interface{}) {
	l.Write(LevelTrace, msg, ctx...)
}

func (l *logger) Debug(msg string, ctx ...interface{}) {
	l.Write(slog.LevelDebug, msg, ctx...)
}

func (l *logger) Info(msg string, ctx ...interface{}) {
	l.Write(slog.LevelInfo, msg, ctx...)
}

func (l *logger) Warn(msg string, ctx ...interface{}) {
	l.Write(slog.LevelWarn, msg, ctx...)
}

func (l *logger) Error(msg string, ctx ...interface{}) {
	l.Write(slog.LevelError, msg, ctx...)
}

func (l *logger) Crit(msg string, ctx ...interface{}) {
	l.Write(LevelCrit, msg, ctx...)
	os.Exit(1)
}

func normalizeAttrs(kv []any) ([]slog.Attr, *slog.Attr) {
	if len(kv) == 0 {
		return nil, nil
	}

	var warnAttr *slog.Attr
	if len(kv)%2 != 0 {
		kv = append(kv, nil)
		attr := slog.String(errorKey, "normalized odd number of arguments by adding nil")
		warnAttr = &attr
	}

	attrs := make([]slog.Attr, 0, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		key := attrKeyString(kv[i])
		attrs = append(attrs, slog.Any(key, kv[i+1]))
	}
	return attrs, warnAttr
}

func attrKeyString(v any) string {
	if v == nil {
		return "nil"
	}
	if key, ok := v.(string); ok {
		return key
	}
	return fmt.Sprint(v)
}

func shortFileName(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}

func captureCaller() (uintptr, []slog.Attr) {
	const maxDepth = 12
	var pcs [maxDepth]uintptr
	n := runtime.Callers(3, pcs[:])
	if n == 0 {
		return 0, nil
	}

	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if shouldSkipFrame(frame.File) {
			if !more {
				break
			}
			continue
		}
		short := shortFileName(frame.File)
		attrs := []slog.Attr{
			slog.String(callerKey, fmt.Sprintf("%s:%d", short, frame.Line)),
			slog.String(fileKey, frame.File),
			slog.Int(lineKey, frame.Line),
		}
		return frame.PC, attrs
	}
	return 0, nil
}

func shouldSkipFrame(file string) bool {
	if file == "" {
		return true
	}
	if strings.Contains(file, "/runtime/") || strings.HasSuffix(file, ".s") {
		return true
	}
	switch filepath.Base(file) {
	case "logger.go", "root.go":
		return true
	default:
		return false
	}
}
