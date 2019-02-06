package get

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-getter"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func Unmarshal(src string, dst interface{}) error {
	bytes, err := GetBytes(src)
	if err != nil {
		return err
	}

	strs := strings.Split(src, "/")
	file := strs[len(strs)-1]
	ext := filepath.Ext(file)

	{
		logrus.Tracef("unmarshalling %s", string(bytes))

		var err error
		switch ext {
		case "json":
			err = json.Unmarshal(bytes, dst)
		default:
			err = yaml.Unmarshal(bytes, dst)
		}

		logrus.Tracef("unmarshalled to %v", dst)

		if err != nil {
			return err
		}
	}

	return nil
}

func GetBytes(goGetterSrc string) ([]byte, error) {
	// This should be shared across variant commands, so that they can share cache for the shared imports
	cacheBaseDir := ".variant"

	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	getterSrcParts := strings.Split(goGetterSrc, "//")
	if len(getterSrcParts) != 2 {
		return nil, fmt.Errorf("format the src description with $repo//$path, like github.com/mumoshu/kodedeploy//kode: %s", goGetterSrc)
	}

	lastIndex := len(getterSrcParts) - 1

	fileAndQuery := strings.SplitN(getterSrcParts[lastIndex], "?", 2)
	file := fileAndQuery[0]
	var fileQuery string
	if len(fileAndQuery) > 1 {
		fileQuery = fileAndQuery[1]
	} else {
		fileQuery = ""
	}

	dirAndQuery := strings.Split(strings.Join(getterSrcParts[:lastIndex], "/"), "?")
	srcDir := dirAndQuery[0]
	var dirQuery string
	if len(dirAndQuery) > 1 {
		dirQuery = dirAndQuery[1]
	} else {
		dirQuery = ""
	}

	query := strings.Join([]string{fileQuery, dirQuery}, "&")

	ctx, cancel := context.WithCancel(context.Background())

	var cacheKey string
	replacer := strings.NewReplacer("/", "_", ".", "_")
	dirKey := replacer.Replace(srcDir)
	if len(query) > 0 {
		paramsKey := strings.Replace(query, "&", "_", -1)
		cacheKey = fmt.Sprintf("%s.%s", dirKey, paramsKey)
	} else {
		cacheKey = dirKey
	}

	cached := false

	dst := filepath.Join(cacheBaseDir, cacheKey)
	{
		stat, err := os.Stat(dst)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat: %v", err)
		} else if err == nil {
			if !stat.IsDir() {
				return nil, fmt.Errorf("%s is not directory. please remove it so that variant could use it for dependency caching", dst)
			}

			cached = true
		}
	}

	if !cached {
		logrus.Debugf("downloading %s to %s", srcDir, dst)

		var src string

		if len(query) == 0 {
			src = srcDir
		} else {
			src = strings.Join([]string{srcDir, query}, "?")
		}

		get := &getter.Client{
			Ctx:     ctx,
			Src:     src,
			Dst:     dst,
			Pwd:     pwd,
			Mode:    getter.ClientModeDir,
			Options: []getter.ClientOption{},
		}

		logrus.Tracef("client: %+v", *get)

		if err := get.Get(); err != nil {
			return nil, fmt.Errorf("get: %v", err)
		}

		cancel()
	}

	bytes, err := ioutil.ReadFile(filepath.Join(dst, file))
	if err != nil {
		return nil, fmt.Errorf("read file: %v", err)
	}

	return bytes, nil
}
