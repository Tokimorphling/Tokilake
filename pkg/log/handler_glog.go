package log

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

var errVmoduleSyntax = errors.New("expect comma-separated list of filename=N")

type GlogHandler struct {
	origin slog.Handler

	level    atomic.Int32
	override atomic.Bool

	patterns  []pattern
	siteCache map[uintptr]slog.Level
	location  string
	lock      sync.RWMutex
}

func NewGlogHandler(h slog.Handler) *GlogHandler {
	return &GlogHandler{origin: h}
}

type pattern struct {
	pattern *regexp.Regexp
	level   slog.Level
}

func (h *GlogHandler) Verbosity(level slog.Level) {
	h.level.Store(int32(level))
}

func (h *GlogHandler) Vmodule(ruleset string) error {
	var filter []pattern
	for _, rule := range strings.Split(ruleset, ",") {
		if len(rule) == 0 {
			continue
		}
		parts := strings.Split(rule, "=")
		if len(parts) != 2 {
			return errVmoduleSyntax
		}
		parts[0] = strings.TrimSpace(parts[0])
		parts[1] = strings.TrimSpace(parts[1])
		if len(parts[0]) == 0 || len(parts[1]) == 0 {
			return errVmoduleSyntax
		}

		l, err := strconv.Atoi(parts[1])
		if err != nil {
			return errVmoduleSyntax
		}
		level := FromLegacyLevel(l)
		if level == LevelCrit {
			continue
		}

		matcher := ".*"
		for _, comp := range strings.Split(parts[0], "/") {
			if comp == "*" {
				matcher += "(/.*)?"
			} else if comp != "" {
				matcher += "/" + regexp.QuoteMeta(comp)
			}
		}
		if !strings.HasSuffix(parts[0], ".go") {
			matcher += "/[^/]+\\.go"
		}
		matcher += "$"

		re, _ := regexp.Compile(matcher)
		filter = append(filter, pattern{pattern: re, level: level})
	}

	h.lock.Lock()
	defer h.lock.Unlock()
	h.patterns = filter
	h.siteCache = make(map[uintptr]slog.Level)
	h.override.Store(len(filter) != 0)
	return nil
}

func (h *GlogHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	return h.override.Load() || slog.Level(h.level.Load()) <= lvl
}

func (h *GlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.lock.RLock()
	siteCache := maps.Clone(h.siteCache)
	h.lock.RUnlock()

	patterns := append([]pattern(nil), h.patterns...)
	res := GlogHandler{
		origin:    h.origin.WithAttrs(attrs),
		patterns:  patterns,
		siteCache: siteCache,
		location:  h.location,
	}
	res.level.Store(h.level.Load())
	res.override.Store(h.override.Load())
	return &res
}

func (h *GlogHandler) WithGroup(name string) slog.Handler {
	panic("not implemented")
}

func (h *GlogHandler) Handle(_ context.Context, r slog.Record) error {
	if slog.Level(h.level.Load()) <= r.Level {
		return h.origin.Handle(context.Background(), r)
	}

	h.lock.RLock()
	lvl, ok := h.siteCache[r.PC]
	h.lock.RUnlock()

	if !ok {
		h.lock.Lock()
		fs := runtime.CallersFrames([]uintptr{r.PC})
		frame, _ := fs.Next()
		for _, rule := range h.patterns {
			if rule.pattern.MatchString(fmt.Sprintf("+%s", frame.File)) {
				h.siteCache[r.PC], lvl, ok = rule.level, rule.level, true
			}
		}
		if !ok {
			h.siteCache[r.PC] = 0
		}
		h.lock.Unlock()
	}
	if lvl <= r.Level {
		return h.origin.Handle(context.Background(), r)
	}
	return nil
}
