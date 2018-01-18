package parse_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jonstaryuk/raven-go"

	"github.com/jonstaryuk/logwatch/parse"
)

const example = `
polygraph/vendor/github.com/uber/jaeger-client-go/log/zap.(*Logger).Error
    /go/src/polygraph/vendor/github.com/uber/jaeger-client-go/log/zap/logger.go:33
polygraph/vendor/github.com/uber/jaeger-client-go.(*remoteReporter).processQueue.func1
    /go/src/polygraph/vendor/github.com/uber/jaeger-client-go/reporter.go:257
`

func TestStacktrace(t *testing.T) {
	expectedFrames := []raven.StacktraceFrame{
		{
			Filename:     "/go/src/polygraph/vendor/github.com/uber/jaeger-client-go/reporter.go",
			Function:     "(*remoteReporter).processQueue.func1",
			Module:       "polygraph/vendor/github.com/uber/jaeger-client-go",
			Lineno:       257,
			AbsolutePath: "/go/src/polygraph/vendor/github.com/uber/jaeger-client-go/reporter.go",
		},
		{
			Filename:     "/go/src/polygraph/vendor/github.com/uber/jaeger-client-go/log/zap/logger.go",
			Function:     "(*Logger).Error",
			Module:       "polygraph/vendor/github.com/uber/jaeger-client-go/log/zap",
			Lineno:       33,
			AbsolutePath: "/go/src/polygraph/vendor/github.com/uber/jaeger-client-go/log/zap/logger.go",
		},
	}
	expected := &raven.Stacktrace{}
	for _, f := range expectedFrames {
		ff := f
		expected.Frames = append(expected.Frames, &ff)
	}

	ze := parse.ZapJSONLogEntry(map[string]interface{}{"stacktrace": example})

	if diff := cmp.Diff(expected, ze.Stacktrace()); diff != "" {
		t.Error(diff)
	}
}
