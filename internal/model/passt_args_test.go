package model

import (
	"reflect"
	"strings"
	"testing"
)

func TestPasstConfigCommandArgs(t *testing.T) {
	cfg := PasstConfig{
		SocketPath: "/srv/vmm/nodes/node-a/run/passt.sock",
		PidPath:    "/srv/vmm/nodes/node-a/run/passt.pid",
	}

	want := []string{"--vhost-user", "--socket", "/srv/vmm/nodes/node-a/run/passt.sock", "--foreground"}
	if got := cfg.CommandArgs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("CommandArgs() = %#v, want %#v", got, want)
	}
	if got := strings.Join(cfg.CommandArgs(), " "); strings.Contains(got, "--pid") {
		t.Fatalf("CommandArgs() = %q, want no --pid", got)
	}
}

func TestPasstConfigString(t *testing.T) {
	cfg := PasstConfig{SocketPath: "/run/passt.sock", PidPath: "/run/passt.pid"}
	if got, want := cfg.String(), "--vhost-user\n--socket /run/passt.sock\n--foreground"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
