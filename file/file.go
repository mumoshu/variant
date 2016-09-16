package file

import (
	"os"
)

func Exists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil

}
