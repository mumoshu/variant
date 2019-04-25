package variant

import (
	"fmt"
	"github.com/mitchellh/colorstring"
	"github.com/sirupsen/logrus"
)

type variantTextFormatter struct {
	colorize *colorstring.Colorize
	colors   map[logrus.Level]string
}

func (f *variantTextFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var prefix = "[" + f.colors[entry.Level] + "]"
	app := entry.Data["app"]
	if app != nil {
		switch app := app.(type) {
		case string:
			task := entry.Data["task"]
			if task != nil {
				switch task := task.(type) {
				case string:
					prefix = fmt.Sprintf("%s%s.%s ≫ ", prefix, app, task)
				}
			} else {
				prefix = fmt.Sprintf("%s%s ≫ ", prefix, app)
			}
		}
	}
	return []byte(f.colorize.Color(fmt.Sprintf("%s%s\n", prefix, entry.Message))), nil
}
