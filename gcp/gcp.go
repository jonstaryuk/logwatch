// Package gcp provides a logwatch.Recorder that forwards entries to Google
// Cloud Logging.
package gcp

import (
	"sync"

	"cloud.google.com/go/logging"

	"github.com/jonstaryuk/logwatch"
)

// A Recorder writes entries to its Client. It uses each entry's Logger() as
// the log ID.
type Recorder struct {
	Client *logging.Client

	loggers map[string]*logging.Logger
	mu      sync.Mutex
}

func (Recorder) Name() string { return "gcp" }

func (r Recorder) Record(le logwatch.Entry, c logwatch.EntryContext) error {
	ge := logging.Entry{
		Timestamp: le.Timestamp(),
		Severity:  logging.ParseSeverity(le.Level()),
		Payload:   le.Data(),
		Labels:    map[string]string{"container": c.ContainerID},
	}

	if ge.Timestamp == logwatch.ZeroTime {
		ge.Timestamp = c.FallbackTime
	}

	name := le.Logger()
	if name == "" {
		name = "default"
	}
	logger, ok := r.loggers[name]
	if !ok {
		if r.loggers == nil {
			r.loggers = make(map[string]*logging.Logger)
		}

		logger = r.Client.Logger(name)

		r.mu.Lock()
		r.loggers[name] = logger
		r.mu.Unlock()
	}

	logger.Log(ge)

	return nil
}

func (r Recorder) Close() error {
	return r.Client.Close()
}
