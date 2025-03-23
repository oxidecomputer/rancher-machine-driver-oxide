package main

import (
	"github.com/oxidecomputer/oxide.go/oxide"
)

// createOxideClient creates an Oxide client from the machine driver
// configuration.
func (d *Driver) createOxideClient() (*oxide.Client, error) {
	return oxide.NewClient(&oxide.Config{
		Host:      d.Host,
		Token:     d.Token,
		UserAgent: "Oxide Rancher Machine Driver/0.0.1 (Go; Linux) [Environment: Development]",
	})
}
