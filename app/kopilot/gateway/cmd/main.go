package main

import (
	"context"

	"github.com/Fl0rencess720/Kopilot/app/kopilot/gateway/internal/conf"
	"github.com/Fl0rencess720/Kopilot/app/kopilot/gateway/internal/pkgs/logging"
	"github.com/Fl0rencess720/Kopilot/app/kopilot/gateway/internal/pkgs/tracing"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
)

var (
	Name = "Kopilot.gateway"
)

func main() {
	conf.Init()
	logging.Init()

	tp, err := tracing.SetTraceProvider(Name)
	if err != nil {
		zap.L().Panic("tracing init err: %s", zap.Error(err))
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			zap.L().Error("trace provider shut down err: %s", zap.Error(err))
		}
	}()

	g := gin.Default()

	p := ginprometheus.NewPrometheus(Name)
	p.Use(g)

	g.Use(gzip.Gzip(gzip.DefaultCompression))

	g.Use(otelgin.Middleware(Name))

}
