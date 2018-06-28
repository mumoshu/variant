package variant

import (
	log "github.com/Sirupsen/logrus"
)

type MessageOnlyFormatter struct {
}

func (f *MessageOnlyFormatter) Format(entry *log.Entry) ([]byte, error) {
	return append([]byte(entry.Message), '\n'), nil
}
