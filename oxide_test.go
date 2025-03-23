// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Copyright 2024 Oxide Computer Company
package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/oxidecomputer/oxide.go/oxide"
	"github.com/rancher/machine/libmachine/state"
)

var _ = Describe("Driver", func() {
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
})
