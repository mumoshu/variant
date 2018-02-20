package logrus

import (
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	"os"
	"time"
)

// See https://github.com/trentm/node-bunyan/blob/cbfaa9a7bd86c658dbb8333c894191d23b65be33/bin/bunyan#L62-L68
var logrusLevelToBunyan map[string]int = map[string]int{
	// "Trace" does not exist in logrus
	//"trace": 10,
	"debug":   20,
	"info":    30,
	"warning": 40,
	"error":   50,
	"fatal":   60,
	// "PANIC" does not exist in bunyan. It is logged as the "LVLpanic" level
}

var (
	pid      int
	hostname string
)

type Formatter struct {
	// TimestampFormat sets the format used for marshaling timestamps.
	TimestampFormat string
	Name            string
}

func init() {
	var err error

	pid = os.Getpid()
	hostname, err = os.Hostname()
	if err != nil {
		hostname = "<hostname n/a>"
	}
}

func (f *Formatter) Format(entry *logrus.Entry) ([]byte, error) {
	data := make(logrus.Fields, len(entry.Data)+3)
	for k, v := range entry.Data {
		switch v := v.(type) {
		case error:
			// Otherwise errors are ignored by `encoding/json`
			// https://github.com/Sirupsen/logrus/issues/137
			data[k] = v.Error()
		default:
			data[k] = v
		}
	}
	prefixFieldClashes(data)

	timestampFormat := f.TimestampFormat
	if timestampFormat == "" {
		timestampFormat = time.RFC3339
	}

	data["time"] = entry.Time.Format(timestampFormat)
	data["msg"] = entry.Message
	data["pid"] = pid
	data["name"] = f.Name
	data["hostname"] = hostname
	data["v"] = 0

	level, ok := logrusLevelToBunyan[entry.Level.String()]
	if ok {
		data["level"] = level
	} else {
		data["level"] = entry.Level.String()
	}

	serialized, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal fields to JSON, %v", err)
	}
	return append(serialized, '\n'), nil
}

// This is to not silently overwrite `time`, `msg` and `level` fields when
// dumping it. If this code wasn't there doing:
//
//  logrus.WithField("level", 1).Info("hello")
//
// Would just silently drop the user provided level. Instead with this code
// it'll logged as:
//
//  {"level": "info", "fields.level": 1, "msg": "hello", "time": "..."}
//
// It's not exported because it's still using Data in an opinionated way. It's to
// avoid code duplication between the two default formatters.
func prefixFieldClashes(data logrus.Fields) {
	if t, ok := data["time"]; ok {
		data["fields.time"] = t
	}

	if m, ok := data["msg"]; ok {
		data["fields.msg"] = m
	}

	if l, ok := data["level"]; ok {
		data["fields.level"] = l
	}
}
