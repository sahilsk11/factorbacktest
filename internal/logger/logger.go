package logger

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// wish list:
// - event keys for finding certain events
// -

func init() {
	var (
		logger *zap.Logger
		err    error
	)
	opts := []zap.Option{
		zap.AddStacktrace(zap.ErrorLevel),
		zap.AddCallerSkip(1),
	}

	if strings.ToLower(os.Getenv("ALPHA_ENV")) == "dev" {
		logger, err = zap.NewDevelopment(opts...)
	} else {
		opts[0] = zap.AddStacktrace(zap.InfoLevel)
		opts = append(opts, zap.Fields(zap.Field{
			Key:    "ALPHA_ENV",
			Type:   zapcore.StringType,
			String: os.Getenv("ALPHA_ENV"),
		}))
		logger, err = zap.NewProduction(opts...)
	}

	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %s", err.Error()))
	}
	zap.ReplaceGlobals(logger)
}

func Info(msg string, args ...interface{}) {
	zap.L().Sugar().Infof(msg, args...)
}

func Warn(msg string, args ...interface{}) {
	zap.L().Sugar().Warnf(msg, args...)
}

func Error(err error, args ...interface{}) {
	a := []interface{}{err}
	a = append(a, args...)
	zap.L().Sugar().Error(a...)
}

func Debug(msg string, args ...interface{}) {
	return // lol i cant figure out how to disable them
	zap.L().Sugar().Debugf(msg, args...)
}
