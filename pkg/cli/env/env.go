package env

import (
	"fmt"
	"io/ioutil"

	log "github.com/sirupsen/logrus"

	"github.com/pkg/errors"
)

type EnvFile struct {
	appName string
}

var e *EnvFile

func init() {
	SetAppName("var")
}

func New(name string) *EnvFile {
	e := new(EnvFile)
	e.appName = name

	return e
}

func SetAppName(name string) {
	e = New(name)
}

func GetPath() string { return e.GetPath() }
func (e *EnvFile) GetPath() string {
	return fmt.Sprintf(".%senv", e.appName)
}

func Set(env string) error { return e.Set(env) }
func (e *EnvFile) Set(env string) error {
	err := ioutil.WriteFile(e.GetPath(), []byte(env), 0644)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func Get() (string, error) { return e.Get() }
func (e *EnvFile) Get() (string, error) {
	env, err := ioutil.ReadFile(e.GetPath())
	if err != nil {
		return "", errors.WithStack(err)
	}
	return string(env), nil
}

func GetOrSet(defaultEnv string) (string, error) { return e.GetOrSet(defaultEnv) }
func (e *EnvFile) GetOrSet(defaultEnv string) (string, error) {
	env, err := e.Get()
	if err != nil {
		log.Debugf("%s", err)
		err := e.Set(defaultEnv)
		if err != nil {
			return "", errors.WithStack(err)
		}
	}
	return env, nil
}
