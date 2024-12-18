// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Copyright 2024 Oxide Computer Company
package main

import (
	"testing"

	"github.com/oxidecomputer/oxide.go/oxide"
	"github.com/rancher/machine/libmachine/state"
	"github.com/stretchr/testify/assert"
)

func TestToRancherMachineState(t *testing.T) {
	tt := []struct {
		name          string
		instanceState oxide.InstanceState
		expectedState state.State
	}{
		{
			name:          "creating",
			instanceState: oxide.InstanceStateCreating,
			expectedState: state.Starting,
		},
		{
			name:          "starting",
			instanceState: oxide.InstanceStateStarting,
			expectedState: state.Starting,
		},
		{
			name:          "running",
			instanceState: oxide.InstanceStateRunning,
			expectedState: state.Running,
		},
		{
			name:          "stopping",
			instanceState: oxide.InstanceStateStopping,
			expectedState: state.Stopping,
		},
		{
			name:          "stopped",
			instanceState: oxide.InstanceStateStopped,
			expectedState: state.Stopped,
		},
		{
			name:          "repairing",
			instanceState: oxide.InstanceStateRepairing,
			expectedState: state.Stopped,
		},
		{
			name:          "rebooting",
			instanceState: oxide.InstanceStateRebooting,
			expectedState: state.Stopping,
		},
		{
			name:          "migrating",
			instanceState: oxide.InstanceStateMigrating,
			expectedState: state.Running,
		},
		{
			name:          "failed",
			instanceState: oxide.InstanceStateFailed,
			expectedState: state.Error,
		},
		{
			name:          "destroyed",
			instanceState: oxide.InstanceStateDestroyed,
			expectedState: state.Error,
		},
		{
			name:          "unknown",
			instanceState: oxide.InstanceState("unknown"),
			expectedState: state.None,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, toRancherMachineState(tc.instanceState), tc.expectedState)
		})
	}
}
