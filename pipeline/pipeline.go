package pipeline

import (
	"time"
)

type DockerJSONLogEntry struct {
	Log    string `json:"log"`
	Stream string `json:"stream"`
	Time   string `json:"time"`
}

type Parser interface {
	Parse(DockerJSONLogEntry) (Entry, error)
}

type Entry interface {
	Level() string
	Message() string
	Timestamp() time.Time         // May return ZeroTime
	Data() map[string]interface{} // Misc structured data
}

type EntryContext struct {
	ContainerID  string    // Docker ID of the container that logged the message
	FallbackTime time.Time // Use this if the Entry returns ZeroTime
}

type Recorder interface {
	Name() string // e.g. "raven"
	Record(Entry, EntryContext) error
	Close() error // Will be called before the program exits
}

var ZeroTime = time.Time{}
