package log

import (
	"bytes"
	"fmt"
	"log/slog"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	timeFormat         = time.RFC3339Nano
	terminalTimeFormat = "2006-01-02T15:04:05.000Z"
	floatFormat        = 'f'
	termMsgJust        = 40
	termCtxMaxPadding  = 40
	callerDisplayWidth = 24
)

var spaces = []byte("                                        ")

type TerminalStringer interface {
	TerminalString() string
}

func (h *TerminalHandler) format(buf []byte, r slog.Record, usecolor bool) []byte {
	msg := escapeMessage(r.Message)
	color := ""
	if usecolor {
		switch r.Level {
		case LevelCrit:
			color = "\x1b[35m"
		case slog.LevelError:
			color = "\x1b[31m"
		case slog.LevelWarn:
			color = "\x1b[33m"
		case slog.LevelInfo:
			color = "\x1b[32m"
		case slog.LevelDebug:
			color = "\x1b[36m"
		case LevelTrace:
			color = "\x1b[34m"
		}
	}
	if buf == nil {
		buf = make([]byte, 0, 30+termMsgJust)
	}
	b := bytes.NewBuffer(buf)

	if color != "" {
		b.WriteString(color)
		b.WriteString(LevelAlignedString(r.Level))
		b.WriteString("\x1b[0m")
	} else {
		b.WriteString(LevelAlignedString(r.Level))
	}
	b.WriteByte(' ')
	writeTimeTermFormat(b, r.Time)

	if caller := h.callerFromRecord(r); caller != "" {
		b.WriteByte(' ')
		b.WriteString(formatCallerDisplay(caller))
	}

	b.WriteString(" - ")
	b.WriteString(msg)

	length := len(msg)
	if (r.NumAttrs()+len(h.attrs)) > 0 && length < termMsgJust {
		b.Write(spaces[:termMsgJust-length])
	}
	h.formatAttributes(b, r, color)
	return b.Bytes()
}

func (h *TerminalHandler) formatAttributes(buf *bytes.Buffer, r slog.Record, color string) {
	writeAttr := func(attr slog.Attr, last bool) {
		buf.WriteByte(' ')
		if color != "" {
			buf.WriteString(color)
			buf.Write(appendEscapeString(buf.AvailableBuffer(), attr.Key))
			buf.WriteString("\x1b[0m=")
		} else {
			buf.Write(appendEscapeString(buf.AvailableBuffer(), attr.Key))
			buf.WriteByte('=')
		}

		val := FormatSlogValue(attr.Value, buf.AvailableBuffer())
		padding := h.fieldPadding[attr.Key]
		length := utf8.RuneCount(val)
		if padding < length && length <= termCtxMaxPadding {
			padding = length
			h.fieldPadding[attr.Key] = padding
		}
		buf.Write(val)
		if !last && padding > length {
			buf.Write(spaces[:padding-length])
		}
	}

	attrs := make([]slog.Attr, 0, len(h.attrs)+r.NumAttrs())
	for _, attr := range h.attrs {
		if shouldSkipAttr(attr.Key) {
			continue
		}
		attrs = append(attrs, attr)
	}
	r.Attrs(func(attr slog.Attr) bool {
		if shouldSkipAttr(attr.Key) {
			return true
		}
		attrs = append(attrs, attr)
		return true
	})

	for i, attr := range attrs {
		writeAttr(attr, i == len(attrs)-1)
	}
	buf.WriteByte('\n')
}

func (h *TerminalHandler) callerFromRecord(r slog.Record) string {
	var caller string
	r.Attrs(func(attr slog.Attr) bool {
		if attr.Key == callerKey {
			caller = attr.Value.String()
			return false
		}
		return true
	})
	if caller != "" {
		return caller
	}
	for _, attr := range h.attrs {
		if attr.Key == callerKey {
			return attr.Value.String()
		}
	}
	return ""
}

func formatCallerDisplay(caller string) string {
	if len(caller) >= callerDisplayWidth {
		return caller[:callerDisplayWidth]
	}
	if callerDisplayWidth-len(caller) > 0 {
		return caller + strings.Repeat(" ", callerDisplayWidth-len(caller))
	}
	return caller
}

func shouldSkipAttr(key string) bool {
	switch key {
	case callerKey, fileKey, lineKey:
		return true
	default:
		return false
	}
}

func FormatSlogValue(v slog.Value, tmp []byte) (result []byte) {
	var value any
	defer func() {
		if err := recover(); err != nil {
			if rv := reflect.ValueOf(value); rv.IsValid() && rv.Kind() == reflect.Pointer && rv.IsNil() {
				result = []byte("<nil>")
				return
			}
			panic(err)
		}
	}()

	switch v.Kind() {
	case slog.KindString:
		return appendEscapeString(tmp, v.String())
	case slog.KindInt64:
		return appendInt64(tmp, v.Int64())
	case slog.KindUint64:
		return appendUint64(tmp, v.Uint64(), false)
	case slog.KindFloat64:
		return strconv.AppendFloat(tmp, v.Float64(), floatFormat, 3, 64)
	case slog.KindBool:
		return strconv.AppendBool(tmp, v.Bool())
	case slog.KindDuration:
		value = v.Duration()
	case slog.KindTime:
		return v.Time().UTC().AppendFormat(tmp, timeFormat)
	default:
		value = v.Any()
	}

	if value == nil {
		return []byte("<nil>")
	}

	switch x := value.(type) {
	case *big.Int:
		return appendBigInt(tmp, x)
	case error:
		return appendEscapeString(tmp, x.Error())
	case TerminalStringer:
		return appendEscapeString(tmp, x.TerminalString())
	case fmt.Stringer:
		return appendEscapeString(tmp, x.String())
	}

	internal := fmt.Appendf(tmp, "%+v", value)
	return appendEscapeString(tmp, string(internal))
}

func appendInt64(dst []byte, n int64) []byte {
	if n < 0 {
		return appendUint64(dst, uint64(-n), true)
	}
	return appendUint64(dst, uint64(n), false)
}

func appendUint64(dst []byte, n uint64, neg bool) []byte {
	if n < 100000 {
		if neg {
			return strconv.AppendInt(dst, -int64(n), 10)
		}
		return strconv.AppendInt(dst, int64(n), 10)
	}

	const maxLength = 26
	out := make([]byte, maxLength)
	i := maxLength - 1
	comma := 0
	for ; n > 0; i-- {
		if comma == 3 {
			comma = 0
			out[i] = ','
		} else {
			comma++
			out[i] = '0' + byte(n%10)
			n /= 10
		}
	}
	if neg {
		out[i] = '-'
		i--
	}
	return append(dst, out[i+1:]...)
}

func FormatLogfmtUint64(n uint64) string {
	return string(appendUint64(nil, n, false))
}

func appendBigInt(dst []byte, n *big.Int) []byte {
	if n.IsUint64() {
		return appendUint64(dst, n.Uint64(), false)
	}
	if n.IsInt64() {
		return appendInt64(dst, n.Int64())
	}

	text := n.String()
	buf := make([]byte, len(text)+len(text)/3)
	comma := 0
	i := len(buf) - 1
	for j := len(text) - 1; j >= 0; j, i = j-1, i-1 {
		c := text[j]
		switch {
		case c == '-':
			buf[i] = c
		case comma == 3:
			buf[i] = ','
			i--
			comma = 0
			fallthrough
		default:
			buf[i] = c
			comma++
		}
	}
	return append(dst, buf[i+1:]...)
}

func appendEscapeString(dst []byte, s string) []byte {
	needsQuoting := false
	needsEscaping := false
	for _, r := range s {
		if r == ' ' || r == '=' {
			needsQuoting = true
			continue
		}
		if r <= '"' || r > '~' {
			needsEscaping = true
			break
		}
	}
	if needsEscaping {
		return strconv.AppendQuote(dst, s)
	}
	if needsQuoting {
		dst = append(dst, '"')
		dst = append(dst, []byte(s)...)
		return append(dst, '"')
	}
	return append(dst, []byte(s)...)
}

func escapeMessage(s string) string {
	needsQuoting := false
	for _, r := range s {
		if r == '\r' || r == '\n' || r == '\t' {
			continue
		}
		if r < ' ' || r > '~' || r == '=' {
			needsQuoting = true
			break
		}
	}
	if !needsQuoting {
		return s
	}
	return strconv.Quote(s)
}

func writeTimeTermFormat(buf *bytes.Buffer, t time.Time) {
	buf.WriteString(t.UTC().Format(terminalTimeFormat))
}
