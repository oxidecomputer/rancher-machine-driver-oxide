package main

import (
	"os"
	"strconv"

	"github.com/rancher/machine/libmachine/log"
)

var debugging = false

func init() {
	env, found := os.LookupEnv("OXIDE_DEBUG")

	if found {
		debugging, _ = strconv.ParseBool(env)
	}
}

func logEntry(msg string) func() {
	if !debugging {
		return func() {}
	}

	log.Debugf(">>> %s", msg)
	return func() {
		log.Debugf("<<< %s", msg)
	}
}
