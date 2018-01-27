package logwatch

import (
	"github.com/jonstaryuk/raven-go"
)

type RavenEntry interface {
	Entry
	Logger() string
	Culprit() string
	Release() string
	Stacktrace() (st *raven.Stacktrace)
}

type RavenRecorder struct {
	Client *raven.Client
}

func NewRavenRecorder(dsn string) (r RavenRecorder, err error) {
	r.Client, err = raven.New(dsn)
	return
}

func (RavenRecorder) Name() string { return "raven" }

func (r RavenRecorder) Record(e Entry, c EntryContext) error {
	level := e.Level()
	if level == "info" || level == "debug" {
		return nil
	}

	p := raven.Packet{
		Message:   e.Message(),
		Timestamp: raven.Timestamp(e.Timestamp()),
		Level:     raven.Severity(level),

		Platform:   "go",
		ServerName: c.ContainerID,

		Extra: e.Data(),
	}

	if p.Timestamp == raven.Timestamp(ZeroTime) {
		p.Timestamp = raven.Timestamp(c.FallbackTime)
		p.Tags = append(p.Tags, raven.Tag{Key: "timestamp_comes_from", Value: "entry_write_time"})
	}

	if re, ok := e.(RavenEntry); ok {
		p.Culprit = re.Culprit()
		p.Release = re.Release()
		p.Interfaces = []raven.Interface{re.Stacktrace()}
	}

	r.Client.Capture(&p, nil)

	return nil
}

func (r RavenRecorder) Close() error {
	r.Client.Wait()
	r.Client.Close()
	return nil
}
