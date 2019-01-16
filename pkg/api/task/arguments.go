package task

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/mumoshu/variant/pkg/util/maputil"
	"github.com/pkg/errors"
	"strings"
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

	log.Debugf("fetching argument named %s in %v", name, in)

	result, internalError := maputil.GetStringAtPath(in, name)
	log.Debugf("failed fetching argument %s: %v", name, internalError)
	if result == "" {
		return nil, nil
	}

	log.WithField("raw", in).Debugf("argument named \"%s\" fetched: %v", name, result)

	if internalError != nil {
		err = errors.WithStack(internalError)
	}

	return result, err
}

func (a Arguments) GetSubOrEmpty(path string) Arguments {
	m, err := maputil.GetValueAtPath(a, strings.Split(path, "."))
	if err != nil {
		panic(err)
	}
	switch m2 := m.(type) {
	case map[interface{}]interface{}:
		strMap, err := maputil.CastKeysToStrings(m2)
		if err != nil {
			panic(err)
		}
		return NewArguments(strMap)
	}
	log.Debugf("no value found for %s in %v", path, a)
	return NewArguments()
}

func (a Arguments) TransformStringValues(f func(v string) string) Arguments {
	r := a.transformStringValues(f)
	strMap, err := maputil.CastKeysToStrings(r)
	if err != nil {
		panic(err)
	}
	return NewArguments(strMap)
}

func (a Arguments) transformStringValues(f func(v string) string) map[interface{}]interface{} {
	r := map[interface{}]interface{}{}
	for k, v := range a {
		switch v2 := v.(type) {
		case map[interface{}]interface{}:
			strMap, err := maputil.CastKeysToStrings(v2)
			if err != nil {
				panic(err)
			}
			r[k] = NewArguments(strMap).transformStringValues(f)
		case string:
			r[k] = f(v2)
		default:
			r[k] = v2
		}
	}
	return r
}
