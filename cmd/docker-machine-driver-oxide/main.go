package main

import (
	"github.com/oxidecomputer/rancher-machine-driver-oxide/pkg/driver/oxide"
	"github.com/rancher/machine/libmachine/drivers/plugin"
)

func main() {
	plugin.RegisterDriver(oxide.NewDriver("", ""))
}
