module github.com/oxidecomputer/rancher-machine-driver-oxide

go 1.22.8

replace (
	github.com/docker/docker => github.com/moby/moby v1.4.2-0.20170731201646-1009e6a40b29
	github.com/docker/machine => github.com/rancher/machine v0.16.2
	github.com/urfave/cli => github.com/urfave/cli v1.11.1-0.20151120215642-0302d3914d2a // newer versions of this will break the rpc binding code
)

require (
	github.com/docker/machine v0.0.0-00010101000000-000000000000
	github.com/go-test/deep v1.1.1
	github.com/oxidecomputer/oxide.go v0.1.0-beta9
	github.com/rancher/machine v0.16.0
	gotest.tools/v3 v3.5.1
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/docker/docker v0.0.0-00010101000000-000000000000 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	golang.org/x/crypto v0.28.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/term v0.25.0 // indirect
)
