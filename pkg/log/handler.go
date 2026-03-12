package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"reflect"
	"sync"
	"time"
)

type discardHandler struct{}

func DiscardHandler() slog.Handler {
	return &discardHandler{}
}

func (h *discardHandler) Handle(_ context.Context, _ slog.Record) error {
	return nil
}

func (h *discardHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return false
}

func (h *discardHandler) WithGroup(name string) slog.Handler {
	panic("not implemented")
}

func (h *discardHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return &discardHandler{}
}

type TerminalHandler struct {
	mu           sync.Mutex
	wr           io.Writer
	lvl          slog.Level
	useColor     bool
	attrs        []slog.Attr
	fieldPadding map[string]int
	buf          []byte
}

func NewTerminalHandler(wr io.Writer, useColor bool) *TerminalHandler {
	return NewTerminalHandlerWithLevel(wr, levelMaxVerbosity, useColor)
}

func NewTerminalHandlerWithLevel(wr io.Writer, lvl slog.Level, useColor bool) *TerminalHandler {
	return &TerminalHandler{
		wr:           wr,
		lvl:          lvl,
		useColor:     useColor,
		fieldPadding: make(map[string]int),
	}
}

func (h *TerminalHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	buf := h.format(h.buf, r, h.useColor)
	_, _ = h.wr.Write(buf)
	h.buf = buf[:0]
	return nil
}

func (h *TerminalHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.lvl
}

func (h *TerminalHandler) WithGroup(name string) slog.Handler {
	panic("not implemented")
}

func (h *TerminalHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TerminalHandler{
		wr:           h.wr,
		lvl:          h.lvl,
		useColor:     h.useColor,
		attrs:        append(h.attrs, attrs...),
		fieldPadding: make(map[string]int),
	}
}

func (h *TerminalHandler) ResetFieldPadding() {
	h.mu.Lock()
	h.fieldPadding = make(map[string]int)
	h.mu.Unlock()
}

type leveler struct{ minLevel slog.Level }

func (l *leveler) Level() slog.Level {
	return l.minLevel
}

func JSONHandler(wr io.Writer) slog.Handler {
	return JSONHandlerWithLevel(wr, levelMaxVerbosity)
}

func JSONHandlerWithLevel(wr io.Writer, level slog.Level) slog.Handler {
	return slog.NewJSONHandler(wr, &slog.HandlerOptions{
		ReplaceAttr: builtinReplaceJSON,
		Level:       &leveler{level},
	})
}

func LogfmtHandler(wr io.Writer) slog.Handler {
	return slog.NewTextHandler(wr, &slog.HandlerOptions{
		ReplaceAttr: builtinReplaceLogfmt,
	})
}

func LogfmtHandlerWithLevel(wr io.Writer, level slog.Level) slog.Handler {
	return slog.NewTextHandler(wr, &slog.HandlerOptions{
		ReplaceAttr: builtinReplaceLogfmt,
		Level:       &leveler{level},
	})
}

func builtinReplaceLogfmt(_ []string, attr slog.Attr) slog.Attr {
	return builtinReplace(nil, attr, true)
}

func builtinReplaceJSON(_ []string, attr slog.Attr) slog.Attr {
	return builtinReplace(nil, attr, false)
}

func builtinReplace(_ []string, attr slog.Attr, logfmt bool) slog.Attr {
	switch attr.Key {
	case slog.TimeKey:
		if attr.Value.Kind() == slog.KindTime {
			tm := attr.Value.Time().UTC()
			if logfmt {
				return slog.String("t", tm.Format(timeFormat))
			}
			return slog.Attr{Key: "t", Value: slog.TimeValue(tm)}
		}
	case slog.LevelKey:
		if l, ok := attr.Value.Any().(slog.Level); ok {
			return slog.Any("lvl", LevelString(l))
		}
	}

	switch v := attr.Value.Any().(type) {
	case time.Time:
		if logfmt {
			return slog.String(attr.Key, v.UTC().Format(timeFormat))
		}
		attr.Value = slog.TimeValue(v.UTC())
	case *big.Int:
		if v == nil {
			attr.Value = slog.StringValue("<nil>")
		} else {
			attr.Value = slog.StringValue(v.String())
		}
	case fmt.Stringer:
		if v == nil || (reflect.ValueOf(v).Kind() == reflect.Pointer && reflect.ValueOf(v).IsNil()) {
			attr.Value = slog.StringValue("<nil>")
		} else {
			attr.Value = slog.StringValue(v.String())
		}
	}
	return attr
}
