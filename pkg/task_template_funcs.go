package variant

import (
	"encoding/json"
	"fmt"
	"github.com/mumoshu/variant/pkg/util/maputil"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"strconv"
	"strings"
)

func join(sep string, arraylike interface{}) (string, error) {
	items, err := toStrings(arraylike)
	if err != nil {
		return "", err
	}
	return strings.Join(items, sep), nil
}

func dig(path string, val interface{}) (interface{}, error) {
	keys := strings.Split(path, ".")
	return _dig(keys, val)
}

func _dig(keys []string, val interface{}) (interface{}, error) {
	if len(keys) == 0 {
		return val, nil
	}
	k := keys[0]
	switch source := val.(type) {
	case map[string]interface{}:
		for key := range source {
			if key == k {
				return _dig(keys[1:], source[k])
			}
		}
		return nil, fmt.Errorf("key \"%s\" not found", k)
	case map[interface{}]interface{}:
		for key := range source {
			switch k2 := key.(type) {
			case string:
				if k2 == k {
					return _dig(keys[1:], source[k])
				}
			default:
				return nil, fmt.Errorf("unexpected type of key: value=%+v, type=%+v", k2, k2)
			}
		}
		return nil, fmt.Errorf("key \"%s\" not found", k)
	default:
		return "", fmt.Errorf("unexpected type of value %+v: %T", source, source)
	}
}

func readFile(path string) (string, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func merge(dst interface{}, srcs ...interface{}) (map[string]interface{}, error) {
	m := map[string]interface{}{}
	return _merge(m, append([]interface{}{dst}, srcs...)...)
}

func _merge(dst map[string]interface{}, srcs ...interface{}) (map[string]interface{}, error) {
	var d map[string]interface{}
	var err error

	if len(srcs) == 0 {
		return dst, nil
	}

	switch dd := srcs[0].(type) {
	case map[string]interface{}:
		d = dd
	case map[interface{}]interface{}:
		d, err = maputil.CastKeysToStrings(dd)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unexpected type error: expected either map[string]interface{} or map[string]string, got %T", dd)
	}

	maputil.DeepMerge(dst, d)

	return _merge(dst, srcs[1:]...)
}

func toJson(val interface{}) (string, error) {
	bytes, err := json.Marshal(val)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func toYaml(val interface{}) (string, error) {
	bytes, err := yaml.Marshal(val)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func fromYaml(str string) map[string]interface{} {
	m := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}

func toStrings(arraylike interface{}) ([]string, error) {
	var items []string
	switch ary := arraylike.(type) {
	case []string:
		items = ary
	case []interface{}:
		items = make([]string, len(ary))
		for i := range ary {
			items[i] = fmt.Sprintf("%s", ary[i])
		}
	default:
		return []string{}, fmt.Errorf("unable to join an (%T): %+v", arraylike, arraylike)
	}
	return items, nil
}

type templateContext struct {
	get func(string) (interface{}, error)
}

func toFlagValue(any interface{}) (string, error) {
	var s string
	switch v := any.(type) {
	case string:
		s = v
	case int:
		s = strconv.Itoa(v)
	case bool:
		if v {
			s = "true"
		} else {
			s = "false"
		}
	case []interface{}:
		ss := make([]string, len(v))
		for i := range v {
			ss[i] = fmt.Sprintf("%s", v[i])
		}
		s = strings.Join(ss, ",")
	default:
		return "", fmt.Errorf("unexpected type for value %+v: %T", v, v)
	}
	return s, nil
}

func (c templateContext) toFlags(val interface{}) (string, error) {
	var m map[string]interface{}
	switch source := val.(type) {
	case map[string]interface{}:
		m = make(map[string]interface{}, len(source))
		for k, v := range source {
			v, err := toFlagValue(v)
			if err != nil {
				return "", err
			}
			m[k] = v
		}
	case map[interface{}]interface{}:
		m = make(map[string]interface{}, len(source))
		for k, v := range source {
			v, err := toFlagValue(v)
			if err != nil {
				return "", err
			}
			m[fmt.Sprintf("%s", k)] = v
		}
	default:
		return "", fmt.Errorf("unexpected type of value %+v: %T", source, source)
	}
	flags := []string{}
	for k, v := range m {
		flags = append(flags, fmt.Sprintf("--%s=%s", k, v))
	}

	return strings.Join(flags, " "), nil
}
