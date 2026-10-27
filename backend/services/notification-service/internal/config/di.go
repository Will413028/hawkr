package config

import (
	"github.com/Will413028/hawkr/go-pkg/logger"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("config",
	fx.Provide(Load),
	fx.Provide(func(cfg *Config) *zap.Logger { return logger.NewLogger(cfg.Env) }),
	fx.Invoke(func(l *zap.Logger) { zap.ReplaceGlobals(l) }),
)
