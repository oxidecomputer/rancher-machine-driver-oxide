// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Copyright 2024 Oxide Computer Company
package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/oxidecomputer/oxide.go/oxide"
	"github.com/rancher/machine/commands/commandstest"
	"github.com/rancher/machine/libmachine/state"
)

var _ = Describe("Driver", func() {
	var SUT *Driver
	var opts *commandstest.FakeFlagger

	BeforeEach(func() {
		SUT = newDriver("bob", "path")
		opts = defaultMockDriverOptions()
	})

	Describe("SetConfigFromFlags", func() {
		It("should succeed when all required fields are given", func() {
			Expect(SUT.SetConfigFromFlags(opts)).To(Succeed())
		})

		Describe("errors", func() {
			DescribeTable("should fail when a required string field is missing",
				func(fields []string) {
					for _, field := range fields {
						opts.Data[field] = ""
					}
					err := SUT.SetConfigFromFlags(opts)
					Expect(err).To(HaveOccurred())
				},
				Entry("host", []string{flagHost}),
				Entry("token", []string{flagToken}),
				Entry("project", []string{flagProject}),
				Entry("diskImageId", []string{flagBootDiskImageID}),
			)

			It("should fail when nothing is given", func() {
				err := SUT.SetConfigFromFlags(&commandstest.FakeFlagger{
					Data: map[string]any{},
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("required option not set: oxide-host"))
				Expect(err.Error()).To(ContainSubstring("required option not set: oxide-token"))
				Expect(err.Error()).To(ContainSubstring("required option not set: oxide-project"))
				Expect(err.Error()).To(ContainSubstring("required option not set: oxide-boot-disk-image-id"))
			})
		})
	})

	DescribeTable("RancherMachineState mapping is correct",
		func(instanceState oxide.InstanceState, expectedState state.State) {
			Expect(toRancherMachineState(instanceState)).To(Equal(expectedState))
		},
		Entry("creating", oxide.InstanceStateCreating, state.Starting),
		Entry("starting", oxide.InstanceStateStarting, state.Starting),
		Entry("running", oxide.InstanceStateRunning, state.Running),
		Entry("stopping", oxide.InstanceStateStopping, state.Stopping),
		Entry("stopped", oxide.InstanceStateStopped, state.Stopped),
		Entry("repairing", oxide.InstanceStateRepairing, state.Starting),
		Entry("rebooting", oxide.InstanceStateRebooting, state.Starting),
		Entry("migrating", oxide.InstanceStateMigrating, state.Running),
		Entry("failed", oxide.InstanceStateFailed, state.Error),
		Entry("destroyed", oxide.InstanceStateDestroyed, state.NotFound),
		Entry("unknown", oxide.InstanceState("unknown"), state.None),
	)

	DescribeTable("Successful ParseAdditionalDisk",
		func(s string, expected AdditionalDisk) {
			Expect(ParseAdditionalDisk(s)).To(Equal(expected))
		},
		Entry("parses integer without suffix", "21474836480", AdditionalDisk{Size: 21474836480, Label: "additional"}),
		Entry("parses integer with suffix", "10GiB", AdditionalDisk{Size: 10737418240, Label: "additional"}),
		Entry("parses integer with space suffix", "10 GiB", AdditionalDisk{Size: 10737418240, Label: "additional"}),
		Entry("parses integer without suffix and label", "21474836480,data", AdditionalDisk{Size: 21474836480, Label: "data"}),
		Entry("parses integer with suffix and label", "10GiB,data", AdditionalDisk{Size: 10737418240, Label: "data"}),
		Entry("parses integer with space suffix and label", "10 GiB,data", AdditionalDisk{Size: 10737418240, Label: "data"}),
		Entry("parses integer without suffix trailing comma", "21474836480,", AdditionalDisk{Size: 21474836480, Label: "additional"}),
		Entry("parses integer with suffix trailing comma", "10GiB,", AdditionalDisk{Size: 10737418240, Label: "additional"}),
	)
})

func defaultMockDriverOptions() (rv *commandstest.FakeFlagger) {
	rv = &commandstest.FakeFlagger{
		Data: map[string]any{},
	}

	rv.Data[flagHost] = "host"
	rv.Data[flagToken] = "token"
	rv.Data[flagProject] = "project"
	rv.Data[flagBootDiskImageID] = "image"

	return rv
}
