package maputil

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"reflect"
	"strconv"
)

func GetValueAtPath(cache map[string]interface{}, keyComponents []string) (interface{}, error) {
	k, rest := keyComponents[0], keyComponents[1:]

	k = strings.Replace(k, "-", "_", -1)

	if len(rest) == 0 {
		return cache[k], nil
	} else {
		nested, ok := cache[k].(map[string]interface{})
		if ok {
			v, err := GetValueAtPath(nested, rest)
			if err != nil {
				return nil, err
			}
			return v, nil
		} else if cache[k] != nil {
			return nil, errors.Errorf("%s is not a map[string]interface{}", k)
		} else {
			return nil, nil
		}
	}
}

func GetStringAtPath(m map[string]interface{}, key string) (string, error) {
	sep := "."

	components := strings.Split(strings.Replace(key, "-", "_", -1), sep)
	head := components[0]
	rest := components[1:]
	value, exists := m[head]
	log.Debugf("fetching %s in %v", key, m)
	if !exists {

		log.Debugf("maptuil fetched %s: %v", key, m[key])

		if value, exists := m[key]; exists {
			if str, ok := value.(string); ok {
				return str, nil
			} else if b, ok := value.(bool); ok {
				return strconv.FormatBool(b), nil
			} else if i, ok := value.(int); ok {
				return strconv.Itoa(i), nil
			} else {
				return "", errors.Errorf("maputil failed to parse string: %v", value)
			}
		}

		return "", fmt.Errorf("no value for %s in %+v", head, m)
	}

	result, isStr := value.(string)

	if !isStr {
		switch next := value.(type) {
		case map[string]interface{}:
			return GetStringAtPath(next, strings.Join(rest, sep))
		case map[interface{}]interface{}:
			converted, err := CastKeysToStrings(next)
			if err != nil {
				return "", err
			}
			return GetStringAtPath(converted, strings.Join(rest, sep))
		default:
			return "", fmt.Errorf("Not map or string: %s in %+v: type is %s", head, m, reflect.TypeOf(next))
		}
	}

	return result, nil
}

func SetValueAtPath(cache map[string]interface{}, keyComponents []string, value interface{}) error {
	k, rest := keyComponents[0], keyComponents[1:]

	k = strings.Replace(k, "-", "_", -1)

	var humanReadableValue string
	if value != nil {
		humanReadableValue = fmt.Sprintf("%#v(%T)", value, value)
	} else {
		humanReadableValue = "<nil>"
	}
	log.Debugf("maptuil sets %v for %s(%s)", humanReadableValue, k, strings.Join(keyComponents, "."))

	if len(rest) == 0 {
		cache[k] = value
	} else {
		_, ok := cache[k].(map[string]interface{})
		if !ok && cache[k] != nil {
			return errors.Errorf("%s is not an map[string]interface{}", k)
		}
		if cache[k] == nil {
			cache[k] = map[string]interface{}{}
		}
		err := SetValueAtPath(cache[k].(map[string]interface{}), rest, value)
		if err != nil {
			return errors.Wrapf(err, "failed setting value for key %+v", keyComponents)
		}
	}
	return nil
}

func FlattenAsString(m map[string]interface{}) string {
	result := ""

	for k, v := range Flatten(m) {
		result = fmt.Sprintf("%s %s=%s", result, k, v)
	}

	return result
}

func Flatten(input map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}

	for k, valOrMap := range input {
		if m, isMap := valOrMap.(map[string]interface{}); isMap {
			for k2, v2 := range Flatten(m) {
				result[fmt.Sprintf("%s.%s", k, k2)] = v2
			}
		} else {
			result[k] = valOrMap
		}
	}

	return result
}

func DeepMerge(dest map[string]interface{}, src map[string]interface{}) {
	for k, v := range src {
		if m, isMap := v.(map[string]interface{}); isMap {
			if _, destIsMap := dest[k].(map[string]interface{}); !destIsMap {
				if d, destIsInterfaceMap := dest[k].(map[interface{}]interface{}); !destIsInterfaceMap {
					if dest[k] != nil {
						log.Panicf("maputil panics! unexpected type of value in map: dest=%v, k=%v, dest[k]=%v", dest, k, dest[k])
					}
					dest[k] = map[string]interface{}{}
				} else {
					ds, err := CastKeysToStrings(d)
					if err != nil {
						log.Panicf("maputil panics! unexpected state of d: %v", d)
					}
					dest[k] = ds
				}
			}
			d, ok := dest[k].(map[string]interface{})

			if !ok {
				log.Panicf("maputil panics! unexpected state of d: %v", d)
			}
			DeepMerge(d, m)
		} else if arr, isArr := v.([]string); isArr {
			dest[k] = arr
		} else {
			dest[k] = v
		}
	}
}

func CastKeysToStrings(m map[interface{}]interface{}) (map[string]interface{}, error) {
	r := map[string]interface{}{}
	for k, v := range m {
		str, ok := k.(string)
		if !ok {
			return nil, fmt.Errorf("Unexpected type %s for key %s", reflect.TypeOf(k), k)
		}
		r[str] = v
	}
	return r, nil
}

// RecursivelyStringifyKeys helps converting any map object into a go-jsonscheme-friendly map
func RecursivelyStringifyKeys(m interface{}) (map[string]interface{}, error) {
	mm, err := _recursivelyStringifyKeys(m)
	if err != nil {
		return nil, err
	}
	if ms, ok := mm.(map[string]interface{}); ok {
		return ms, nil
	}
	return nil, fmt.Errorf("bug: unexpected type of m: %T", mm)
}

func _recursivelyStringifyKeys(m interface{}) (interface{}, error) {
	switch src := m.(type) {
	case map[string]interface{}:
		dst := map[string]interface{}{}
		for k, v1 := range src {
			v2, err := _recursivelyStringifyKeys(v1)
			if err != nil {
				return nil, err
			}
			dst[k] = v2
		}
		return dst, nil
	case []interface{}:
		dst := make([]interface{}, len(src))
		for i, v1 := range src {
			v2, err := _recursivelyStringifyKeys(v1)
			if err != nil {
				return nil, err
			}
			dst[i] = v2
		}
		return dst, nil
	case map[interface{}]interface{}:
		dst := map[string]interface{}{}
		for k1, v1 := range src {
			k2, ok := k1.(string)
			if !ok {
				return nil, fmt.Errorf("unexpected type of key \"%v\": %T", k1, k1)
			}
			v2, err := _recursivelyStringifyKeys(v1)
			if err != nil {
				return nil, err
			}
			dst[k2] = v2
		}
		return dst, nil
	}
	return m, nil
}
