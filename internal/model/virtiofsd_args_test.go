package model

import (
	"reflect"
	"testing"
)

func TestVirtiofsConfigCommandArgs(t *testing.T) {
	tests := []struct {
		name string
		cfg  VirtiofsConfig
		want []string
	}{
		{
			name: "without socket group",
			cfg: VirtiofsConfig{
				Directory:      "/srv/vmm/nodes/a/configuration",
				SocketPath:     "/srv/vmm/nodes/a/run/virtiofs_configuration.sock",
				ThreadPoolSize: 2,
			},
			want: []string{
				"--allow-direct-io",
				"--announce-submounts",
				"--writeback",
				"--xattr",
				"--posix-acl",
				"--thread-pool-size", "2",
				"--cache", "auto",
				"--inode-file-handles=prefer",
				"--shared-dir", "/srv/vmm/nodes/a/configuration",
				"--modcaps", "+sys_admin",
				"--xattrmap", ":map::user.virtiofs.:",
				"--socket-path", "/srv/vmm/nodes/a/run/virtiofs_configuration.sock",
			},
		},
		{
			name: "with socket group",
			cfg: VirtiofsConfig{
				Directory:      "/srv/vmm/nodes/a/configuration",
				SocketPath:     "/srv/vmm/nodes/a/run/virtiofs_configuration.sock",
				SocketGroup:    "hvm",
				ThreadPoolSize: 4,
			},
			want: []string{
				"--allow-direct-io",
				"--announce-submounts",
				"--writeback",
				"--xattr",
				"--posix-acl",
				"--thread-pool-size", "4",
				"--cache", "auto",
				"--inode-file-handles=prefer",
				"--shared-dir", "/srv/vmm/nodes/a/configuration",
				"--modcaps", "+sys_admin",
				"--xattrmap", ":map::user.virtiofs.:",
				"--socket-path", "/srv/vmm/nodes/a/run/virtiofs_configuration.sock",
				"--socket-group", "hvm",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.CommandArgs(); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("CommandArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestVirtiofsConfigString(t *testing.T) {
	cfg := VirtiofsConfig{
		Directory:      "/share",
		SocketPath:     "/run/virtiofs_share.sock",
		SocketGroup:    "hvm",
		ThreadPoolSize: 2,
	}

	want := "--allow-direct-io\n--announce-submounts\n--writeback\n--xattr\n--posix-acl\n--thread-pool-size 2\n--cache auto\n--inode-file-handles=prefer\n--shared-dir /share\n--modcaps +sys_admin\n--xattrmap :map::user.virtiofs.:\n--socket-path /run/virtiofs_share.sock\n--socket-group hvm"
	if got := cfg.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
