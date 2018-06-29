package task

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
	"github.com/mumoshu/variant/pkg/util/maputil"
)

type Arguments map[string]interface{}

func NewArguments(raw ...map[string]interface{}) Arguments {
	if len(raw) == 0 {
		return Arguments(map[string]interface{}{})
	} else if len(raw) == 1 {
		return Arguments(raw[0])
	} else {
		panic(fmt.Sprintf("bug! unexpected number of args to NewArguments: %d", len(raw)))
	}
}

func (in Arguments) Get(name string) (interface{}, error) {
	var err error

	result, internalError := maputil.GetStringAtPath(in, name)
	if result == "" {
		return nil, nil
	}

	log.WithField("raw", in).Debugf("argument named \"%s\" fetched: %v", name, result)

	if internalError != nil {
		err = errors.Trace(internalError)
	}

	return result, err
}
