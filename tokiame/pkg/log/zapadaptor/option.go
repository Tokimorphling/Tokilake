package zapadaptor

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ExtraKey string

type Option interface {
	apply(cfg *config)
}

type option func(cfg *config)

func (fn option) apply(cfg *config) {
	fn(cfg)
}

type coreConfig struct {
	enc zapcore.Encoder
	ws  zapcore.WriteSyncer
	lvl zap.AtomicLevel
}

type traceConfig struct {
	recordStackTraceInSpan bool
	errorSpanLevel         zapcore.Level
}

type config struct {
	customFields  []interface{}
	extraKeys     []ExtraKey
	coreConfig    coreConfig
	zapOpts       []zap.Option
	traceConfig   *traceConfig
	extraKeyAsStr bool
}

func consoleCoreConfig() *coreConfig {
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.RFC3339TimeEncoder
	encCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	encCfg.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.UTC().Format(time.RFC3339))
	}

	enc := zapcore.NewConsoleEncoder(encCfg)
	return &coreConfig{
		enc: enc,
		ws:  zapcore.AddSync(os.Stderr),
		lvl: zap.NewAtomicLevelAt(zap.DebugLevel),
	}
}

// defaultCoreConfig default zapcore config: json encoder, atomic level, stdout write syncer
func defaultCoreConfig() *coreConfig {
	// default log encoder
	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	// default log level
	lvl := zap.NewAtomicLevelAt(zap.InfoLevel)
	// default write syncer stdout
	ws := zapcore.AddSync(os.Stdout)

	return &coreConfig{
		enc: enc,
		ws:  ws,
		lvl: lvl,
	}
}

func defaultConsoleConfig() *config {
	coreConfig := consoleCoreConfig()
	return &config{
		coreConfig: *coreConfig,
		traceConfig: &traceConfig{
			recordStackTraceInSpan: true,
			errorSpanLevel:         zapcore.ErrorLevel,
		},
		zapOpts:       []zap.Option{},
		extraKeyAsStr: false,
		customFields:  []interface{}{},
	}
}

func defaultConfig() *config {
	coreConfig := defaultCoreConfig()

	return &config{
		coreConfig: *coreConfig,
		traceConfig: &traceConfig{
			recordStackTraceInSpan: true,
			errorSpanLevel:         zapcore.ErrorLevel,
		},
		zapOpts:       []zap.Option{},
		extraKeyAsStr: false,
		customFields:  []interface{}{},
	}
}

// WithCoreEnc zapcore encoder
func WithCoreEnc(enc zapcore.Encoder) Option {
	return option(func(cfg *config) {
		cfg.coreConfig.enc = enc
	})
}

// WithCoreWs zapcore write syncer
func WithCoreWs(ws zapcore.WriteSyncer) Option {
	return option(func(cfg *config) {
		cfg.coreConfig.ws = ws
	})
}

// WithCoreLevel zapcore log level
func WithCoreLevel(lvl zap.AtomicLevel) Option {
	return option(func(cfg *config) {
		cfg.coreConfig.lvl = lvl
	})
}

// WithCustomFields record log with the key-value pair.
func WithCustomFields(kv ...interface{}) Option {
	return option(func(cfg *config) {
		cfg.customFields = append(cfg.customFields, kv...)
	})
}

// WithZapOptions add origin zap option
func WithZapOptions(opts ...zap.Option) Option {
	return option(func(cfg *config) {
		cfg.zapOpts = append(cfg.zapOpts, opts...)
	})
}

// WithTraceErrorSpanLevel trace error span level option
func WithTraceErrorSpanLevel(level zapcore.Level) Option {
	return option(func(cfg *config) {
		cfg.traceConfig.errorSpanLevel = level
	})
}

// WithRecordStackTraceInSpan record stack track option
func WithRecordStackTraceInSpan(recordStackTraceInSpan bool) Option {
	return option(func(cfg *config) {
		cfg.traceConfig.recordStackTraceInSpan = recordStackTraceInSpan
	})
}

// WithExtraKeys allow you log extra values from context
func WithExtraKeys(keys []ExtraKey) Option {
	return option(func(cfg *config) {
		for _, k := range keys {
			if !inArray(k, cfg.extraKeys) {
				cfg.extraKeys = append(cfg.extraKeys, k)
			}
		}
	})
}

// WithExtraKeyAsStr convert extraKey to a string type when retrieving value from context
// Not recommended for use, only for compatibility with certain situations
//
// For more information, refer to the documentation at
// `https://pkg.go.dev/context#WithValue`
func WithExtraKeyAsStr() Option {
	return option(func(cfg *config) {
		cfg.extraKeyAsStr = true
	})
}
