package logwatch_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonstaryuk/raven-go"

	"github.com/jonstaryuk/logwatch"
)

func TestZapEntryParser(t *testing.T) {
	de := logwatch.DockerJSONLogEntry{
		Log:    "{\"level\":\"error\",\"ts\":1516231863.1643379,\"logger\":\"scraper\",\"caller\":\"zap/logger.go:33\",\"msg\":\"test\",\"release\":\"abc123\",\"stacktrace\":\"polygraph/vendor/github.com/uber/jaeger-client-go/log/zap.(*Logger).Error\\n\\t/go/src/polygraph/vendor/github.com/uber/jaeger-client-go/log/zap/logger.go:33\\npolygraph/vendor/github.com/uber/jaeger-client-go.(*remoteReporter).processQueue.func1\\n\\t/go/src/polygraph/vendor/github.com/uber/jaeger-client-go/reporter.go:257\\npolygraph/vendor/github.com/uber/jaeger-client-go.(*remoteReporter).processQueue\\n\\t/go/src/polygraph/vendor/github.com/uber/jaeger-client-go/reporter.go:267\"}\n",
		Stream: "stderr",
		Time:   "2018-01-17T23:31:03.164678054Z",
	}
	// t1 := time.Date(1, 0, 0, 0, 0, 0, 0, time.Local)
	// c := logwatch.EntryContext{ContainerID: "foobar", FallbackTime: t1}
	actual, err := logwatch.ZapJSONLogEntryParser{}.Parse(de)
	if err != nil {
		t.Fatal(err)
	}

	expected := &logwatch.ZapJSONLogEntry{
		"level":                "error",
		"ts":                   1516231863.1643379,
		"logger":               "scraper",
		"caller":               "zap/logger.go:33",
		"msg":                  "test",
		"release":              "abc123",
		"stacktrace":           "polygraph/vendor/github.com/uber/jaeger-client-go/log/zap.(*Logger).Error\n\t/go/src/polygraph/vendor/github.com/uber/jaeger-client-go/log/zap/logger.go:33\npolygraph/vendor/github.com/uber/jaeger-client-go.(*remoteReporter).processQueue.func1\n\t/go/src/polygraph/vendor/github.com/uber/jaeger-client-go/reporter.go:257\npolygraph/vendor/github.com/uber/jaeger-client-go.(*remoteReporter).processQueue\n\t/go/src/polygraph/vendor/github.com/uber/jaeger-client-go/reporter.go:267",
		"timestamp_comes_from": "zap_entry",
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error(diff)
	}
}

func TestZapEntry(t *testing.T) {
	var e logwatch.RavenEntry = &logwatch.ZapJSONLogEntry{
		"level":                "error",
		"ts":                   1516231863.0,
		"logger":               "scraper",
		"caller":               "zap/logger.go:33",
		"msg":                  "test",
		"release":              "abc123",
		"stacktrace":           "polygraph/vendor/github.com/uber/jaeger-client-go/log/zap.(*Logger).Error\n\t/go/src/polygraph/vendor/github.com/uber/jaeger-client-go/log/zap/logger.go:33\npolygraph/vendor/github.com/uber/jaeger-client-go.(*remoteReporter).processQueue.func1\n\t/go/src/polygraph/vendor/github.com/uber/jaeger-client-go/reporter.go:257",
		"timestamp_comes_from": "zap_entry",
	}

	for _, diff := range []string{
		cmp.Diff("error", e.Level()),
		cmp.Diff("test", e.Message()),
		cmp.Diff(time.Unix(1516231863, 0), e.Timestamp()),
		cmp.Diff("zap/logger.go:33", e.Culprit()),
		cmp.Diff("abc123", e.Release()),
	} {
		if diff != "" {
			t.Error(diff)
		}
	}

	expfs := []raven.StacktraceFrame{
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
	expst := &raven.Stacktrace{}
	for _, f := range expfs {
		ff := f
		expst.Frames = append(expst.Frames, &ff)
	}
	if diff := cmp.Diff(expst, e.Stacktrace()); diff != "" {
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

// 	ze := logwatch.ZapJSONLogEntry(unmarshaled)
// 	actual := (&ze).RavenPacket("job123")

// 	if diff := cmp.Diff(expected, *actual, cmpopts.IgnoreFields(raven.Packet{}, "Timestamp")); diff != "" {
// 		t.Error(diff)
// 	}

// 	if time.Time(expected.Timestamp) != time.Time(actual.Timestamp) {
// 		t.Error(time.Time(expected.Timestamp), time.Time(actual.Timestamp))
// 	}
// }
