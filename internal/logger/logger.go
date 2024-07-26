package logger

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New() *zap.SugaredLogger {
	var (
		logger *zap.Logger
		err    error
	)
	opts := []zap.Option{
		zap.AddStacktrace(zap.ErrorLevel),
		// zap.AddCallerSkip(1),
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
		panic(fmt.Errorf("failed to initialize logger: %w", err))
	}

	return logger.Sugar()
}

const ContextKey = "LOGGER"

func FromContext(ctx context.Context) *zap.SugaredLogger {
	logger, ok := ctx.Value(ContextKey).(*zap.SugaredLogger)
	if !ok {
		logger := New()
		logger.Warn("no logger found in ctx - creating new one")
	}
	return logger
}

func init() {
	logger := New()
	zap.ReplaceGlobals(logger.Desugar())
}
