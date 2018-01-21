// Package observer provides tools for collecting and parsing logs written by
// Docker's `json-file` logging driver.
package observer // import "github.com/jonstaryuk/logwatch/observer"

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/hpcloud/tail"
	"github.com/jonstaryuk/raven-go"
	"go.uber.org/zap"

	"github.com/jonstaryuk/logwatch/parse"
)

// An Observer watches a Docker metadata directory, tails the log files of
// running containers, parses log entries as they come in, and forwards them
// to other services.
type Observer struct {
	Sentry *raven.Client
	Done   chan bool
	Logger *zap.SugaredLogger

	containerWatcher *fsnotify.Watcher
}

// New creates and starts an Observer for the given directory, which should
// typically be /var/lib/docker/containers.
func New(dir string, sentry *raven.Client, log *zap.SugaredLogger) (*Observer, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := watcher.Add(dir); err != nil {
		return nil, err
	}

	o := &Observer{
		containerWatcher: watcher,
		Sentry:           sentry,
		Done:             make(chan bool),
		Logger:           log,
	}

	// Find existing directories
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("Reading dir to find existing container dirs: %v", err)
	}
	for _, file := range files {
		if file.IsDir() {
			path := path.Join(dir, file.Name())
			if err := o.tail(path, 0); err != nil {
				o.Logger.Errorw(err.Error(), "loadingExistingDir", path)
			}
		}
	}

	// Watch for new ones
	go o.observe()

	return o, nil
}

// Close closes the underlying directory watcher.
func (o *Observer) Close() error {
	return o.containerWatcher.Close()
}

// observe blocks waiting for filesystem events in the directory.
func (o *Observer) observe() {
	o.Logger.Debug("Starting observer")

	defer func() { o.Done <- true }()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		o.Done <- true
	}()

	for {
		select {
		case e := <-o.containerWatcher.Events:
			if e.Name == "" {
				continue
			}

			o.Logger.Debugf("Received event: %#v", e)
			if e.Op&fsnotify.Create != fsnotify.Create {
				continue
			}

			f, err := os.Open(e.Name)
			if err != nil {
				o.Logger.Error(err)
				continue
			}

			stat, err := f.Stat()
			if err != nil {
				o.Logger.Error(err)
				continue
			}

			if err := f.Close(); err != nil {
				o.Logger.Error(err)
				continue
			}

			if stat.Mode().IsDir() {
				if err := o.tail(e.Name, 1*time.Second); err != nil {
					o.Logger.Error(err)
				}
			}

		case err := <-o.containerWatcher.Errors:
			if err != nil {
				o.Logger.Error(err)
			}
		}
	}
}

// tail starts a goroutine that tails the container's JSON log file in the
// given directory.
func (o *Observer) tail(dirpath string, wait time.Duration) error {
	containerID := path.Base(dirpath)
	logfilename := fmt.Sprintf(path.Join(dirpath, containerID+"-json.log"))

	time.Sleep(wait)

	// Ensure the json log file exists
	if f, err := os.Open(logfilename); err != nil {
		if os.IsNotExist(err) {
			o.Logger.Infof("Container logfile %#v does not exist", logfilename)
			return nil
		}
		return err
	} else {
		f.Close()
	}

	t, err := tail.TailFile(logfilename, tail.Config{
		MustExist: true,
		Follow:    true,
		ReOpen:    true,
		// Location:  &tail.SeekInfo{Offset: 0, Whence: os.SEEK_END},
	})
	if err != nil {
		return err
	}

	go func() {
		o.Logger.Debugf("Tailing %v", logfilename)

		for line := range t.Lines {
			o.Logger.Debugf("Got line: %v", line)

			if line.Err != nil {
				o.Logger.Error(fmt.Errorf("Tailing %v: %v", logfilename, line.Err))
				continue
			}

			if err := o.record(line, containerID); err != nil {
				o.Logger.Error(err)
			}

			o.Logger.Debug("Recorded line")
		}

		o.Logger.Debug("Done tailing %v", logfilename)
		t.Cleanup()
	}()

	return nil
}

// record handles a new log entry.
func (o *Observer) record(l *tail.Line, containerID string) error {
	var de dockerJSONLogEntry
	if err := json.Unmarshal([]byte(l.Text), &de); err != nil {
		return fmt.Errorf("Unmarshaling Docker log entry: %v", err)
	}

	var ze parse.ZapJSONLogEntry
	if err := json.Unmarshal([]byte(de.Log), &ze); err != nil {
		return fmt.Errorf("Unmarshaling Zap log entry from Docker entry: %v", err)
	}

	level := ze.GetString("level")
	if level == "info" || level == "debug" {
		return nil
	}

	p := ze.RavenPacket(containerID)

	// If there was no timestamp in the JSON log entry, fall back to the
	// Docker entry timestamp
	if p.Timestamp == zeroTime {
		ts, err := time.Parse(time.RFC3339Nano, de.Time)
		if err == nil {
			p.Timestamp = raven.Timestamp(ts)
			p.Tags = append(p.Tags, raven.Tag{Key: "timestamp_comes_from", Value: "docker_entry"})
		}
	}
	// Otherwise, fall back to the tail time.
	if p.Timestamp == zeroTime {
		p.Timestamp = raven.Timestamp(l.Time)
		p.Tags = append(p.Tags, raven.Tag{Key: "timestamp_comes_from", Value: "tail_time"})
	}

	o.Logger.Debug("Capturing packet: %#v", p)
	o.Sentry.Capture(p, nil)

	return nil
}

type dockerJSONLogEntry struct {
	Log    string `json:"log"`
	Stream string `json:"stream"`
	Time   string `json:"time"`
}

var zeroTime = raven.Timestamp(time.Time{})
