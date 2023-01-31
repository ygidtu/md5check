package main

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewEncoderConfig as name says
func NewEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		// Keys can be anything except the empty string.
		TimeKey:        "T",
		LevelKey:       "L",
		NameKey:        "N",
		CallerKey:      "C",
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

// TimeEncoder as name says
func TimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
}

func setLogger(debug bool) *zap.SugaredLogger {

	level := zap.InfoLevel
	encoder := NewEncoderConfig()
	if debug {
		level = zap.DebugLevel
	}

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoder),
		zapcore.AddSync(os.Stderr),
		level,
	)

	logger := zap.New(core, zap.AddCaller())
	defer logger.Sync()
	return logger.Sugar()
}
