package internal

import (
	"io/ioutil"
	"os"
)

func ReadSecretEnv(name string) string {
	env := os.Getenv(name)
	if env != "" {
		return env
	}

	file := os.Getenv(name + "_FILE")
	if file == "" {
		return ""
	}

	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return ""
	}

	return string(bytes)
}
