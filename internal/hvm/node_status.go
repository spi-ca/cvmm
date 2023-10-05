package hvm

import (
	"errors"
)

type NodeStatus string

const (
	NodeStatusInvalid  NodeStatus = ""
	NodeStatusCreated  NodeStatus = "Created"
	NodeStatusRunning  NodeStatus = "Running"
	NodeStatusShutdown NodeStatus = "Shutdown"
	NodeStatusPaused   NodeStatus = "Paused"
)

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

func (ts NodeStatus) String() string {
	if ts.IsValid() {
		return string(ts)
	} else {
		return ""
	}
}

func (ts NodeStatus) MarshalText() ([]byte, error) { return []byte(ts.String()), nil }

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
