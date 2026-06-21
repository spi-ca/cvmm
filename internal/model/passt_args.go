package model

// PasstConfig describes the node-scoped host-side passt helper runtime paths and command shape.
// PidPath is cvmm-owned bookkeeping for the direct child PID and is not passed to passt via --pid.
type PasstConfig struct {
	SocketPath string
	PidPath    string
}

// CommandArgs renders the passt flags for the managed vhost-user socket.
func (p PasstConfig) CommandArgs() []string {
	return []string{"--vhost-user", "--socket", p.SocketPath, "--foreground"}
}

// String renders the passt command arguments in the same log-friendly format as VM arguments.
func (p PasstConfig) String() string { return joinArgs(p.CommandArgs()) }
