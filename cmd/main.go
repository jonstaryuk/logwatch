package main

import (
	"context"
	"flag"
	"io/ioutil"
	"os"
	"strings"

	"cloud.google.com/go/logging"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"

	"github.com/jonstaryuk/logwatch"
	"github.com/jonstaryuk/logwatch/gcp"
)

var config struct {
	Dev          bool
	SentryDsn    string `required:"true" split_words:"true"`
	GcpProjectId string `required:"true" split_words:"true"`
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
	ravenRecorder, err := logwatch.NewRavenRecorder(dsn)
	if err != nil {
		panic(err)
	}

	gcpClient, err := logging.NewClient(context.Background(), config.GcpProjectId)
	if err != nil {
		panic(err)
	}

	obs, err := logwatch.NewObserver(dir)
	if err != nil {
		panic(err)
	}
	defer obs.Close()

	obs.Parser = logwatch.ZapJSONLogEntryParser{}
	obs.Recorders = []logwatch.Recorder{
		ravenRecorder,
		gcp.Recorder{Client: gcpClient},
	}
	obs.Logger = log
	obs.Debug = config.Dev

	go obs.Observe()
	<-obs.Done()
}
