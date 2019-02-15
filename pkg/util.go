package variant

import (
	"log"
	"reflect"
	"github.com/mumoshu/variant/pkg/util/maputil"
)

func getOrDefault(nillable interface{}, kind reflect.Kind, defValue interface{}) interface{} {
	if nillable != nil {
		v := reflect.ValueOf(nillable)
		k := v.Kind()
		if k != kind {
			log.Fatalf("unexpected kind: expected %v, but got %v", kind, k)
		}
		switch k {
		case reflect.String:
			return v.String()
		case reflect.Int:
			return v.Int()
		case reflect.Bool:
			return v.Bool()
		case reflect.Map:
			m, err := maputil.RecursivelyStringifyKeys(v.Interface())
			if err != nil {
				panic(err)
			}
			return m
		case reflect.Slice:
			return v.Interface()
		default:
			log.Fatalf("unsupported kind %v", k)
		}
	}
	return defValue
}
