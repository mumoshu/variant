package fileutil

import (
	"os"
)

func Exists(filename string) bool {
	stat, err := os.Stat(filename)
	return err == nil && !stat.IsDir()
}
