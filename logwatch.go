package main

import (
	"flag"
	"io/ioutil"
	"os"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"

	"github.com/jonstaryuk/logwatch/observer"
	"github.com/jonstaryuk/logwatch/pipeline"
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
	zlog, err := zcfg.Build()
	if err != nil {
		panic(err)
	}
	log := zlog.Sugar().Named("logwatch")

	if !config.Dev {
		data, err := ioutil.ReadFile("/commit.sha")
		if err != nil {
			log.Warnf("Could not read /commit.sha: %v", err)
		} else {
			log = log.With("release", string(data))
		}
	}

	var dsn string
	if strings.HasPrefix(config.SentryDsn, "https://") {
		// DSN literal
		dsn = config.SentryDsn
	} else {
		// Not a DSN literal, assume it's a path to a file that contains the DSN
		data, err := ioutil.ReadFile(config.SentryDsn)
		if err != nil {
			panic(err)
		}
		dsn = strings.TrimSpace(string(data))
	}
	ravenRecorder, err := pipeline.NewRavenRecorder(dsn)
	if err != nil {
		panic(err)
	}

	obs, err := observer.New(dir)
	if err != nil {
		panic(err)
	}
	defer obs.Close()

	obs.Parser = pipeline.ZapJSONLogEntryParser{}
	obs.Recorders = []pipeline.Recorder{ravenRecorder}
	obs.Logger = log
	obs.Debug = config.Dev

	go obs.Observe()
	<-obs.Done()
}
