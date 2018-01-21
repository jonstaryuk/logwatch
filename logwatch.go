package main

import (
	"flag"
	"os"

	"github.com/jonstaryuk/raven-go"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"

	"github.com/jonstaryuk/logwatch/observer"
)

var config struct {
	Dev       bool
	SentryDsn string `required:"true" split_words:"true"`
}

func main() {
	envconfig.MustProcess("logwatch", &config)

	flag.Parse()
	dir := flag.Arg(0)

	if dir == "" {
		println("usage: logwatch <dir>")
		os.Exit(2)
	}

	var zcfg zap.Config
	if config.Dev {
		zcfg = zap.NewDevelopmentConfig()
	} else {
		zcfg = zap.NewProductionConfig()
	}
	log, err := zcfg.Build()
	if err != nil {
		panic(err)
	}

	sentry, err := raven.New(config.SentryDsn)
	if err != nil {
		panic(err)
	}
	defer sentry.Close()
	defer sentry.Wait()

	obs, err := observer.New(dir, sentry, log.Sugar().Named("logwatch"))
	if err != nil {
		panic(err)
	}
	defer obs.Close()

	<-obs.Done
}
