module github.com/oxidecomputer/rancher-machine-driver-oxide

go 1.23.6

replace (
	github.com/docker/docker => github.com/moby/moby v1.4.2-0.20170731201646-1009e6a40b29
	github.com/urfave/cli => github.com/urfave/cli v1.11.1-0.20151120215642-0302d3914d2a // Newer versions will break the RPC binding code.
)

require (
	github.com/dustin/go-humanize v1.0.1
	github.com/onsi/ginkgo/v2 v2.22.0
	github.com/onsi/gomega v1.36.1
	github.com/oxidecomputer/oxide.go v0.3.0
	github.com/rancher/machine v0.15.0-rancher122
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/docker/docker v0.7.3-0.20190327010347-be7ac8be2ae0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/pprof v0.0.0-20241029153458-d1b30febd7db // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/urfave/cli v1.22.15 // indirect
	golang.org/x/crypto v0.35.0 // indirect
	golang.org/x/net v0.36.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/term v0.29.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	golang.org/x/tools v0.26.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
