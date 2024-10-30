package oxide

import (
	"testing"

	"github.com/docker/machine/libmachine/state"
	"github.com/oxidecomputer/oxide.go/oxide"
	"gotest.tools/v3/assert"
)

func TestOxideToDockerMachineState(t *testing.T) {
	tests := []struct {
		name     string
		input    oxide.InstanceState
		expected state.State
	}{
		{"Creating", oxide.InstanceStateCreating, state.Starting},
		{"Starting", oxide.InstanceStateStarting, state.Starting},
		{"Running", oxide.InstanceStateRunning, state.Running},
		{"Stopping", oxide.InstanceStateStopping, state.Stopping},
		{"Stopped", oxide.InstanceStateStopped, state.Stopped},
		{"Repairing", oxide.InstanceStateRepairing, state.Stopped},
		{"Rebooting", oxide.InstanceStateRebooting, state.Stopping},
		{"Migrating", oxide.InstanceStateMigrating, state.Running},
		{"Failed", oxide.InstanceStateFailed, state.Error},
		{"Destroyed", oxide.InstanceStateDestroyed, state.Error},
		{"Unknown", oxide.InstanceState("unknown"), state.None},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := OxideToDockerMachineState(tt.input)
			assert.Equal(t, actual, tt.expected)
		})
	}
}
