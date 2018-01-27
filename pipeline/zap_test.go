package pipeline_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	// "github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonstaryuk/raven-go"

	"github.com/jonstaryuk/logwatch/pipeline"
)

func TestZapStacktrace(t *testing.T) {
	exampleStacktrace := `
	polygraph/vendor/github.com/uber/jaeger-client-go/log/zap.(*Logger).Error
	    /go/src/polygraph/vendor/github.com/uber/jaeger-client-go/log/zap/logger.go:33
	polygraph/vendor/github.com/uber/jaeger-client-go.(*remoteReporter).processQueue.func1
	    /go/src/polygraph/vendor/github.com/uber/jaeger-client-go/reporter.go:257
	`

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

	ze := pipeline.ZapJSONLogEntry(map[string]interface{}{"stacktrace": exampleStacktrace})

	if diff := cmp.Diff(expected, ze.Stacktrace()); diff != "" {
		t.Error(diff)
	}
}

// func TestZapRavenPacket(t *testing.T) {
// 	entry := `{"level":"error","ts":1516491077,"logger":"foobar","caller":"baz/qux.go:33","msg":"oh no","release":"abcdefg"}`
// 	unmarshaled := map[string]interface{}{}
// 	if err := json.Unmarshal([]byte(entry), &unmarshaled); err != nil {
// 		t.Fatal(err)
// 	}
// 	expected := raven.Packet{
// 		Message:   "oh no",
// 		Timestamp: raven.Timestamp(time.Unix(1516491077, 0)),

// 		Level:  raven.ERROR,
// 		Logger: "foobar",

// 		Platform:   "go",
// 		Culprit:    "baz/qux.go:33",
// 		ServerName: "job123",
// 		Release:    "abcdefg",
// 		Extra:      unmarshaled,
// 		Tags:       []raven.Tag{{Key: "timestamp_comes_from", Value: "zap_entry"}},

// 		Interfaces: []raven.Interface{&raven.Stacktrace{Frames: []*raven.StacktraceFrame{}}},
// 	}

// 	ze := pipeline.ZapJSONLogEntry(unmarshaled)
// 	actual := (&ze).RavenPacket("job123")

// 	if diff := cmp.Diff(expected, *actual, cmpopts.IgnoreFields(raven.Packet{}, "Timestamp")); diff != "" {
// 		t.Error(diff)
// 	}

// 	if time.Time(expected.Timestamp) != time.Time(actual.Timestamp) {
// 		t.Error(time.Time(expected.Timestamp), time.Time(actual.Timestamp))
// 	}
// }
