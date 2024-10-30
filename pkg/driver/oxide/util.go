package oxide

import (
	"fmt"
	"github.com/docker/machine/libmachine/state"
	"github.com/oxidecomputer/oxide.go/oxide"
)

type requiredOptionError string

func (r requiredOptionError) Error() string {
	return fmt.Sprintf("Oxide driver requires the %q option.", string(r))
}

func OxideToDockerMachineState(runState oxide.InstanceState) state.State {
	switch runState {
	case oxide.InstanceStateCreating, oxide.InstanceStateStarting:
		return state.Starting
	case oxide.InstanceStateRunning:
		return state.Running
	case oxide.InstanceStateStopping:
		return state.Stopping
	case oxide.InstanceStateStopped, oxide.InstanceStateRepairing:
		return state.Stopped
	case oxide.InstanceStateRebooting:
		// TODO: verify
		return state.Stopping
	case oxide.InstanceStateMigrating:
		// TODO: verify
		return state.Running
	case oxide.InstanceStateFailed, oxide.InstanceStateDestroyed:
		return state.Error
	default:
		return state.None
	}
}
