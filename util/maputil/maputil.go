package maputil

import (
	"fmt"
	"strings"

	"github.com/juju/errors"
	"log"
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
	if !exists {
		return "", fmt.Errorf("No value for %s in %+v", head, m)
	}

	next, isMap := value.(map[string]interface{})
	result, isStr := value.(string)

	if !isStr {
		if !isMap {
			return "", fmt.Errorf("Not map or string: %s in %+v", head, m)
		}

		if len(rest) == 0 {
			return "", fmt.Errorf("%s in %+v is a map but no more key to recurse", head, m)
		}

		return GetStringAtPath(next, strings.Join(rest, sep))
	}

	return result, nil
}

func SetValueAtPath(cache map[string]interface{}, keyComponents []string, value interface{}) error {
	k, rest := keyComponents[0], keyComponents[1:]

	k = strings.Replace(k, "-", "_", -1)

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
			return errors.Trace(err)
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

func Flatten(input map[string]interface{}) map[string]string {
	result := map[string]string{}

	for k, strOrMap := range input {
		if str, isStr := strOrMap.(string); isStr {
			result[k] = str
		} else if m, isMap := strOrMap.(map[string]interface{}); isMap {
			for k2, v2 := range Flatten(m) {
				result[fmt.Sprintf("%s.%s", k, k2)] = v2
			}
		} else if a, isArray := strOrMap.([]string); isArray {
			result[k] = strings.Join(a, ",")
		} else {
			log.Panicf("maputil panics! unexpected type of value in map: input=%v, k=%v, v=%v", input, k, strOrMap)
		}
	}

	return result
}

func DeepMerge(dest map[string]interface{}, src map[string]interface{}) {
	for k, v := range src {
		if str, isStr := v.(string); isStr {
			dest[k] = str
		} else if arr, isArr := v.([]string); isArr {
			dest[k] = arr
		} else if m, isMap := v.(map[string]interface{}); isMap {
			if _, destIsMap := dest[k].(map[string]interface{}); !destIsMap {
				if dest[k] != nil {
					log.Panicf("maputil manics! unexpected type of value in map: dest=%v, k=%v, dest[k]=%v", dest, k, dest[k])
				}
				dest[k] = map[string]interface{}{}
			}
			d, ok := dest[k].(map[string]interface{})

			if !ok {
				log.Panicf("maputil panics! unexpected state of d: %v", d)
			}
			DeepMerge(d, m)
		} else {
			log.Panicf("maputil panics! unexpected type of value in map: src=%v, k=%v, v=%v", src, k, v)
		}
	}
}
