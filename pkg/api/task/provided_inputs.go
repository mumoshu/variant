package task

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
	"github.com/mumoshu/variant/pkg/util/maputil"
)

type ProvidedInputs map[string]interface{}

func NewProvidedInputs(raw ...map[string]interface{}) ProvidedInputs {
	if len(raw) == 0 {
		return ProvidedInputs(map[string]interface{}{})
	} else if len(raw) == 1 {
		return ProvidedInputs(raw[0])
	} else {
		panic(fmt.Sprintf("bug! unexpected number of args to NewProvidedInputs: %d", len(raw)))
	}
}

func (in ProvidedInputs) Get(key string) (string, error) {
	var err error

	result, internalError := maputil.GetStringAtPath(in, key)

	log.WithField("raw", in).Debugf("provided input fetched %s: %v", key, result)

	if internalError != nil {
		err = errors.Trace(internalError)
	}

	return result, err
}
