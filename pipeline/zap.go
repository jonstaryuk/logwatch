package pipeline

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jonstaryuk/raven-go"
)

type ZapJSONLogEntryParser struct{}

func (zp ZapJSONLogEntryParser) Parse(de DockerJSONLogEntry) (Entry, error) {
	ze := &ZapJSONLogEntry{}
	if err := json.Unmarshal([]byte(de.Log), ze); err != nil {
		return nil, fmt.Errorf("Unmarshaling Zap log entry: %v", err)
	}

	// If there was no timestamp in the zap log entry, fall back to the Docker
	// entry timestamp
	ts, err := time.Parse(time.RFC3339Nano, de.Time)
	if err == nil {
		(*ze)["ts"] = ts.UnixNano()
		(*ze)["timestamp_comes_from"] = "docker_entry"
	}

	return ze, nil
}

type ZapJSONLogEntry map[string]interface{}

var _ RavenEntry = &ZapJSONLogEntry{} // ensure it implements RavenEntry

func (ze *ZapJSONLogEntry) get(key string) string {
	value, ok := (*ze)[key]
	if !ok {
		return ""
	}

	str, ok := value.(string)
	if !ok {
		return ""
	}

	return str
}

func (ze *ZapJSONLogEntry) Level() string { return ze.get("level") }

func (ze *ZapJSONLogEntry) Message() string { return ze.get("msg") }

func (ze *ZapJSONLogEntry) Data() map[string]interface{} { return *ze }

func (ze *ZapJSONLogEntry) Timestamp() time.Time {
	if ts, ok := (*ze)["ts"]; ok {
		if tsfloat, ok := ts.(float64); ok {
			t := time.Unix(int64(tsfloat), int64(tsfloat*1000000000)%1000000000)
			(*ze)["timestamp_comes_from"] = "zap_entry"
			return t
		}
	}

	return ZeroTime
}

func (ze *ZapJSONLogEntry) Culprit() string { return ze.get("caller") }

func (ze *ZapJSONLogEntry) Release() string { return ze.get("release") }

func (ze *ZapJSONLogEntry) Logger() string { return ze.get("logger") }

func (ze *ZapJSONLogEntry) Stacktrace() (st *raven.Stacktrace) {
	st = &raven.Stacktrace{Frames: []*raven.StacktraceFrame{}}

	lines := strings.Split(strings.TrimSpace(ze.get("stacktrace")), "\n")
	if len(lines) == 0 || len(lines)%2 == 1 {
		return
	}

	for i := 0; i < len(lines); i += 2 {
		var f raven.StacktraceFrame

		frameinfo := strings.Split(strings.TrimSpace(lines[i]), "/")
		funcinfo := strings.SplitN(frameinfo[len(frameinfo)-1], ".", 2)
		f.Module = strings.Join(frameinfo[:len(frameinfo)-1], "/") + "/" + funcinfo[0]
		if len(funcinfo) > 1 {
			f.Function = funcinfo[1]
		}

		sourceinfo := strings.SplitN(strings.TrimSpace(lines[i+1]), ":", 2)
		f.AbsolutePath = sourceinfo[0]
		if len(sourceinfo) > 1 {
			if n, err := strconv.Atoi(sourceinfo[1]); err == nil {
				f.Lineno = n
			}
		}
		// path := strings.Split(f.AbsolutePath, "/")
		// f.Filename = path[len(path)-1]
		f.Filename = sourceinfo[0]

		st.Frames = append(st.Frames, &f)
	}

	// Sentry expects stack frames in reverse order. Reverse the slice.
	n := len(st.Frames)
	for i := 0; i < n/2; i++ {
		f := st.Frames[i]
		st.Frames[i] = st.Frames[n-i-1]
		st.Frames[n-i-1] = f
	}

	return
}
