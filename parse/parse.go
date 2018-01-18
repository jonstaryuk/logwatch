package parse // import "github.com/jonstaryuk/logwatch/parse"

import (
	"strconv"
	"strings"

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

		frameinfo := strings.Split(lines[i], "/")
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
