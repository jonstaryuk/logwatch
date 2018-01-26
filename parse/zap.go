package parse // import "github.com/jonstaryuk/logwatch/parse"

import (
	"strconv"
	"strings"
	"time"

	"github.com/jonstaryuk/raven-go"
)

type ZapJSONLogEntry map[string]interface{}

func (ze *ZapJSONLogEntry) GetString(key string) string {
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

func (ze *ZapJSONLogEntry) Stacktrace() (st *raven.Stacktrace) {
	st = &raven.Stacktrace{Frames: []*raven.StacktraceFrame{}}

	lines := strings.Split(strings.TrimSpace(ze.GetString("stacktrace")), "\n")
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

func (ze *ZapJSONLogEntry) RavenPacket(containerID string) *raven.Packet {
	p := raven.Packet{
		Message: ze.GetString("msg"),

		Level:  raven.Severity(ze.GetString("level")),
		Logger: ze.GetString("logger"),

		Platform:   "go",
		Culprit:    ze.GetString("caller"),
		ServerName: containerID,
		Release:    ze.GetString("release"),
		Extra:      *ze,

		Interfaces: []raven.Interface{ze.Stacktrace()},
	}

	if ts, ok := (*ze)["ts"]; ok {
		if tsfloat, ok := ts.(float64); ok {
			t := time.Unix(int64(tsfloat), int64(tsfloat*1000000000)%1000000000)
			p.Timestamp = raven.Timestamp(t)
			p.Tags = append(p.Tags, raven.Tag{Key: "timestamp_comes_from", Value: "zap_entry"})
		}
	}

	return &p
}
