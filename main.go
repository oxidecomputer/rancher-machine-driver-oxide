package main

import (
	"github.com/rancher/machine/libmachine/drivers/plugin"
)

func main() {
	plugin.RegisterDriver(newDriver("", ""))
}
