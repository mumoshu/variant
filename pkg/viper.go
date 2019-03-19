package variant

import "github.com/spf13/viper"

type Viper interface {
	Get(key string) interface{}
	Sub(key string) *viper.Viper
}
