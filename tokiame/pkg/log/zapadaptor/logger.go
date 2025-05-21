package zapadaptor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"tokiame/pkg/log"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	traceIDKey    = "trace_id"
	spanIDKey     = "span_id"
	traceFlagsKey = "trace_flags"
)

type Logger struct {
	*zap.SugaredLogger
	config *config
}

func NewLogger(ops ...Option) *Logger {
	config := defaultConfig()
	for _, opt := range ops {
		opt.apply(config)
	}

	logger := zap.New(
		zapcore.NewCore(config.coreConfig.enc, config.coreConfig.ws, config.coreConfig.lvl),
		config.zapOpts...)
	return &Logger{
		SugaredLogger: logger.Sugar().With(config.customFields...),
		config:        config,
	}
}

func NewConsoleLogger(ops ...Option) *Logger {
	config := defaultConsoleConfig()
	for _, opt := range ops {
		opt.apply(config)
	}
	logger := zap.New(
		zapcore.NewCore(config.coreConfig.enc, config.coreConfig.ws, config.coreConfig.lvl),
		config.zapOpts...,
	)
	return &Logger{
		SugaredLogger: logger.Sugar().With(config.customFields...),
		config:        config,
	}
}

func (l *Logger) GetExtraKeys() []ExtraKey {
	return l.config.extraKeys
}

func (l *Logger) PutExtraKeys(keys ...ExtraKey) {
	for _, k := range keys {
		if !inArray(k, l.config.extraKeys) {
			l.config.extraKeys = append(l.config.extraKeys, k)
		}
	}
}

func (l *Logger) Print(v ...any) {
	fmt.Print(v...)
}

func (l *Logger) Log(level log.Level, kvs ...interface{}) {
	logger := l.With()

	switch level {
	case log.LevelTrace, log.LevelDebug:
		logger.Debug(kvs...)
	case log.LevelInfo:
		logger.Info(kvs...)
	case log.LevelNotice, log.LevelWarn:
		logger.Warn(kvs...)
	case log.LevelError:
		logger.Error(kvs...)
	case log.LevelFatal:
		logger.Fatal(kvs...)
	default:
		logger.Warn(kvs...)
	}
}

// func (l *Logger) Desugared() *zap.Logger {
// 	return l.SugaredLogger.Desugar()
// }

func (l *Logger) Logf(level log.Level, format string, kvs ...interface{}) {
	logger := l.With()

	switch level {
	case log.LevelTrace, log.LevelDebug:
		logger.Debugf(format, kvs...)
	case log.LevelInfo:
		logger.Infof(format, kvs...)
	case log.LevelNotice, log.LevelWarn:
		logger.Warnf(format, kvs...)
	case log.LevelError:
		logger.Errorf(format, kvs...)
	case log.LevelFatal:
		logger.Fatalf(format, kvs...)
	default:
		logger.Warnf(format, kvs...)
	}
}

func (l *Logger) CtxLogf(level log.Level, ctx context.Context, format string, kvs ...interface{}) {
	var zlevel zapcore.Level
	var sl *zap.SugaredLogger

	span := trace.SpanFromContext(ctx)
	var traceKVs []interface{}
	if span.SpanContext().TraceID().IsValid() {
		traceKVs = append(traceKVs, traceIDKey, span.SpanContext().TraceID())
	}
	if span.SpanContext().SpanID().IsValid() {
		traceKVs = append(traceKVs, spanIDKey, span.SpanContext().SpanID())
	}
	if span.SpanContext().TraceFlags().IsSampled() {
		traceKVs = append(traceKVs, traceFlagsKey, span.SpanContext().TraceFlags())
	}
	if len(traceKVs) > 0 {
		sl = l.With(traceKVs...)
	} else {
		sl = l.With()
	}

	if len(l.config.extraKeys) > 0 {
		for _, k := range l.config.extraKeys {
			if l.config.extraKeyAsStr {
				sl = sl.With(string(k), ctx.Value(string(k)))
			} else {
				sl = sl.With(string(k), ctx.Value(k))
			}
		}
	}

	switch level {
	case log.LevelDebug, log.LevelTrace:
		zlevel = zap.DebugLevel
		sl.Debugf(format, kvs...)
	case log.LevelInfo:
		zlevel = zap.InfoLevel
		sl.Infof(format, kvs...)
	case log.LevelNotice, log.LevelWarn:
		zlevel = zap.WarnLevel
		sl.Warnf(format, kvs...)
	case log.LevelError:
		zlevel = zap.ErrorLevel
		sl.Errorf(format, kvs...)
	case log.LevelFatal:
		zlevel = zap.FatalLevel
		sl.Fatalf(format, kvs...)
	default:
		zlevel = zap.WarnLevel
		sl.Warnf(format, kvs...)
	}

	if !span.IsRecording() {
		return
	}

	// set span status
	if zlevel >= l.config.traceConfig.errorSpanLevel {
		msg := getMessage(format, kvs)
		span.SetStatus(codes.Error, "")
		span.RecordError(errors.New(msg), trace.WithStackTrace(l.config.traceConfig.recordStackTraceInSpan))
	}
}

// func (l *Logger) Print(v ...any) {
// 	// l.Print(log.LevelTrace, v...)
// }

func (l *Logger) Trace(v ...interface{}) {
	l.Log(log.LevelTrace, v...)
}

func (l *Logger) Debug(v ...interface{}) {
	l.Log(log.LevelDebug, v...)
}

func (l *Logger) Info(v ...interface{}) {
	l.Log(log.LevelInfo, v...)
}

func (l *Logger) Notice(v ...interface{}) {
	l.Log(log.LevelNotice, v...)
}

func (l *Logger) Warn(v ...interface{}) {
	l.Log(log.LevelWarn, v...)
}

func (l *Logger) Error(v ...interface{}) {
	l.Log(log.LevelError, v...)
}

func (l *Logger) Fatal(v ...interface{}) {
	l.Log(log.LevelFatal, v...)
}

func (l *Logger) Tracef(format string, v ...interface{}) {
	l.Logf(log.LevelTrace, format, v...)
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	l.Logf(log.LevelDebug, format, v...)
}

func (l *Logger) Infof(format string, v ...interface{}) {
	l.Logf(log.LevelInfo, format, v...)
}

func (l *Logger) Noticef(format string, v ...interface{}) {
	l.Logf(log.LevelInfo, format, v...)
}

func (l *Logger) Warnf(format string, v ...interface{}) {
	l.Logf(log.LevelWarn, format, v...)
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	l.Logf(log.LevelError, format, v...)
}

func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.Logf(log.LevelFatal, format, v...)
}

func (l *Logger) CtxTracef(ctx context.Context, format string, v ...interface{}) {
	l.CtxLogf(log.LevelDebug, ctx, format, v...)
}

func (l *Logger) CtxDebugf(ctx context.Context, format string, v ...interface{}) {
	l.CtxLogf(log.LevelDebug, ctx, format, v...)
}

func (l *Logger) CtxInfof(ctx context.Context, format string, v ...interface{}) {
	l.CtxLogf(log.LevelInfo, ctx, format, v...)
}

func (l *Logger) CtxNoticef(ctx context.Context, format string, v ...interface{}) {
	l.CtxLogf(log.LevelWarn, ctx, format, v...)
}

func (l *Logger) CtxWarnf(ctx context.Context, format string, v ...interface{}) {
	l.CtxLogf(log.LevelWarn, ctx, format, v...)
}

func (l *Logger) CtxErrorf(ctx context.Context, format string, v ...interface{}) {
	l.CtxLogf(log.LevelError, ctx, format, v...)
}

func (l *Logger) CtxFatalf(ctx context.Context, format string, v ...interface{}) {
	l.CtxLogf(log.LevelFatal, ctx, format, v...)
}

func (l *Logger) SetLevel(level log.Level) {
	var lvl zapcore.Level
	switch level {
	case log.LevelTrace, log.LevelDebug:
		lvl = zap.DebugLevel
	case log.LevelInfo:
		lvl = zap.InfoLevel
	case log.LevelWarn, log.LevelNotice:
		lvl = zap.WarnLevel
	case log.LevelError:
		lvl = zap.ErrorLevel
	case log.LevelFatal:
		lvl = zap.FatalLevel
	default:
		lvl = zap.WarnLevel
	}
	l.config.coreConfig.lvl.SetLevel(lvl)
}

func (l *Logger) SetOutput(writer io.Writer) {
	ws := zapcore.AddSync(writer)
	log := zap.New(
		zapcore.NewCore(l.config.coreConfig.enc, ws, l.config.coreConfig.lvl),
		l.config.zapOpts...,
	)
	l.config.coreConfig.ws = ws
	l.SugaredLogger = log.Sugar().With(l.config.customFields...)
}

// Logger is used to return an instance of *zap.Logger for custom fields, etc.
func (l *Logger) Logger() *zap.Logger {
	return l.SugaredLogger.Desugar()
}

func (l *Logger) CtxKVLog(ctx context.Context, level log.Level, format string, kvs ...interface{}) {
	if len(kvs) == 0 || len(kvs)%2 != 0 {
		l.Warn(fmt.Sprint("Keyvalues must appear in pairs:", kvs))
		return
	}

	span := trace.SpanFromContext(ctx)
	if span.SpanContext().TraceID().IsValid() {
		kvs = append(kvs, traceIDKey, span.SpanContext().TraceID())
	}
	if span.SpanContext().SpanID().IsValid() {
		kvs = append(kvs, spanIDKey, span.SpanContext().SpanID())
	}
	if span.SpanContext().TraceFlags().IsSampled() {
		kvs = append(kvs, traceFlagsKey, span.SpanContext().TraceFlags())
	}

	var zlevel zapcore.Level
	zl := l.With()
	switch level {
	case log.LevelDebug, log.LevelTrace:
		zlevel = zap.DebugLevel
		zl.Debugw(format, kvs...)
	case log.LevelInfo:
		zlevel = zap.InfoLevel
		zl.Infow(format, kvs...)
	case log.LevelNotice, log.LevelWarn:
		zlevel = zap.WarnLevel
		zl.Warnw(format, kvs...)
	case log.LevelError:
		zlevel = zap.ErrorLevel
		zl.Errorw(format, kvs...)
	case log.LevelFatal:
		zlevel = zap.FatalLevel
		zl.Fatalw(format, kvs...)
	default:
		zlevel = zap.WarnLevel
		zl.Warnw(format, kvs...)
	}

	if !span.IsRecording() {
		return
	}

	if zlevel >= l.config.traceConfig.errorSpanLevel {
		msg := getMessage(format, kvs)
		span.SetStatus(codes.Error, "")
		span.RecordError(errors.New(msg), trace.WithStackTrace(l.config.traceConfig.recordStackTraceInSpan))
	}
}

func (l *Logger) ToGinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Next()
		cost := time.Since(start)
		l.Desugar().Info(path,
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			// zap.String("user-agent", c.Request.UserAgent()),
			zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
			zap.Duration("cost", cost),
		)
	}
}

func (l *Logger) ToGinRecovery(stack bool) gin.HandlerFunc {
	logger := l.Desugar()
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") || strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}
				httpRequest, _ := httputil.DumpRequest(c.Request, false)
				if brokenPipe {
					logger.Error(c.Request.URL.Path,
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
					c.Error(err.(error)) // nolint: errcheck
					c.Abort()
					return
				}

				if stack {
					logger.Error("[Recovery from panic]",
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
						zap.String("stack", string(debug.Stack())),
					)
				} else {
					logger.Error("[Recovery from panic]",
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
				}
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
