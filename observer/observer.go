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

	"github.com/jonstaryuk/logwatch/pipeline"
)

// An Observer watches a Docker metadata directory, tails the log files of
// running containers, parses log entries as they come in, and forwards them
// to other services.
type Observer struct {
	Parser    pipeline.Parser
	Recorders []pipeline.Recorder

	// Meta-logging observer events
	Logger *zap.SugaredLogger
	Debug  bool

	dir     string
	watcher *fsnotify.Watcher
	done    chan bool
}

// New creates an Observer for the given directory, which should typically be
// /var/lib/docker/containers.
func New(dir string) (*Observer, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := watcher.Add(dir); err != nil {
		return nil, err
	}

	o := &Observer{
		dir:     dir,
		watcher: watcher,
		done:    make(chan bool),
	}

	return o, nil
}

// Close closes the underlying directory watcher and Recorders.
func (o *Observer) Close() {
	for _, r := range o.Recorders {
		if err := r.Close(); err != nil {
			o.Logger.Errorf("closing %s recorder: %v", r.Name(), err)
		}
	}

	if err := o.watcher.Close(); err != nil {
		o.Logger.Errorf("closing directory watcher: %v", err)
	}
}

func (o *Observer) Done() chan bool {
	return o.done
}

// Observe blocks waiting for filesystem events in the directory.
func (o *Observer) Observe() {
	if len(o.Recorders) == 0 {
		panic("No recorders specified")
	}

	if o.Logger == nil {
		o.Logger = zap.NewNop().Sugar()
	}

	o.Logger.Debug("Starting observer")

	defer func() { o.done <- true }()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		o.done <- true
	}()

	// Find existing directories
	files, err := ioutil.ReadDir(o.dir)
	if err != nil {
		panic("Reading dir to find existing container dirs: " + err.Error())
	}
	for _, file := range files {
		if file.IsDir() {
			path := path.Join(o.dir, file.Name())
			if err := o.tail(path, 0, false); err != nil {
				o.Logger.Errorw(err.Error(), "loadingExistingDir", path)
			}
		}
	}

	// Watch for new ones
	for {
		select {
		case e := <-o.watcher.Events:
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
				if err := o.tail(e.Name, time.Second, true); err != nil {
					o.Logger.Error(err)
				}
			}

		case err := <-o.watcher.Errors:
			if err != nil {
				o.Logger.Error(err)
			}
		}
	}
}

// tail starts a goroutine that tails the container's JSON logfile in 'dir',
// waiting 'wait' before attempting to open the logfile. If existing is true,
// any existing log entries in the file will also be parsed.
func (o *Observer) tail(dir string, wait time.Duration, existing bool) error {
	containerID := path.Base(dir)
	logfilename := fmt.Sprintf(path.Join(dir, containerID+"-json.log"))

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

	logger := o.Logger.With("logfile", logfilename)

	tcfg := tail.Config{MustExist: true, Follow: true, ReOpen: true}
	if !existing {
		tcfg.Location = &tail.SeekInfo{Offset: 0, Whence: os.SEEK_END}
	}
	if !o.Debug {
		tcfg.Logger = tail.DiscardingLogger
	}

	t, err := tail.TailFile(logfilename, tcfg)
	if err != nil {
		return err
	}

	go func() {
		logger.Debugf("Starting tail")

		for line := range t.Lines {
			logger.Debugf("Got line: %v", line)

			if line.Err != nil {
				logger.Error(line.Err)
				continue
			}

			if err := o.record(line, containerID); err != nil {
				logger.Error(err)
			}

			logger.Debug("Recorded line")
		}

		logger.Debug("Done tailing")
		t.Cleanup()
	}()

	return nil
}

// record handles a new log entry.
func (o *Observer) record(l *tail.Line, containerID string) error {
	var de pipeline.DockerJSONLogEntry
	if err := json.Unmarshal([]byte(l.Text), &de); err != nil {
		return fmt.Errorf("Unmarshaling Docker log entry: %v", err)
	}

	e, err := o.Parser.Parse(de)
	if err != nil {
		return fmt.Errorf("parser: %v", err)
	}

	c := pipeline.EntryContext{
		ContainerID:  containerID,
		FallbackTime: l.Time,
	}

	for _, r := range o.Recorders {
		if err := r.Record(e, c); err != nil {
			o.Logger.Errorf("%s recorder: %v", r.Name(), err)
		}
	}

	return nil
}

var zeroTime = raven.Timestamp(time.Time{})
