package model

import (
	"errors"
)

// NodeStatus is an enum-like value used when translating cvmm input or cloud-hypervisor state.
type NodeStatus string

const (
	// NodeStatusInvalid is the zero value used when parsing fails.
	NodeStatusInvalid NodeStatus = ""
	// NodeStatusCreated means the VM exists but has not booted.
	NodeStatusCreated NodeStatus = "Created"
	// NodeStatusRunning means the VM is currently running.
	NodeStatusRunning NodeStatus = "Running"
	// NodeStatusShutdown means the VM has shut down.
	NodeStatusShutdown NodeStatus = "Shutdown"
	// NodeStatusPaused means the VM is paused.
	NodeStatusPaused NodeStatus = "Paused"
)

// NodeStatusNameOf parses a cloud-hypervisor VM state name.
func NodeStatusNameOf(value string) (NodeStatus, error) {
	switch value {
	case string(NodeStatusCreated):
		return NodeStatusCreated, nil
	case string(NodeStatusRunning):
		return NodeStatusRunning, nil
	case string(NodeStatusShutdown):
		return NodeStatusShutdown, nil
	case string(NodeStatusPaused):
		return NodeStatusPaused, nil
	default:
		return NodeStatusInvalid, errors.New("unable to find NodeStatusInvalid")
	}
}

// IsValid reports whether the receiver is one of the supported values.
func (ts NodeStatus) IsValid() bool {
	switch ts {
	case NodeStatusCreated:
		return true
	case NodeStatusRunning:
		return true
	case NodeStatusShutdown:
		return true
	case NodeStatusPaused:
		return true
	default:
		return false
	}
}

// String returns the cloud-hypervisor text token for a valid NodeStatus.
func (ts NodeStatus) String() string {
	if ts.IsValid() {
		return string(ts)
	} else {
		return ""
	}
}

// MarshalText returns the textual representation used by encoders.
func (ts NodeStatus) MarshalText() ([]byte, error) { return []byte(ts.String()), nil }

// UnmarshalText parses a textual value into the receiver.
func (ts *NodeStatus) UnmarshalText(b []byte) error {
	length := len(b)
	if length < 0 {
		return errors.New("malformed format")
	}

	new_status, err := NodeStatusNameOf(string(b))
	if err != nil {
		return err
	}

	*ts = new_status
	return nil
}
