package model

import (
	"strconv"
)

type VirtiofsConfig struct {
	Directory      string
	SocketPath     string
	SocketGroup    string
	ThreadPoolSize int
}

func (i *VirtiofsConfig) CommandArgs() []string {
	var args []string
	args = append(args, "--allow-direct-io")
	args = append(args, "--announce-submounts")
	args = append(args, "--writeback")
	args = append(args, "--xattr")
	args = append(args, "--posix-acl")
	args = append(args, "--thread-pool-size", strconv.Itoa(i.ThreadPoolSize))
	args = append(args, "--cache", "auto")
	args = append(args, "--inode-file-handles=prefer")
	args = append(args, "--shared-dir", i.Directory)
	args = append(args, "--modcaps", "+sys_admin")
	args = append(args, "--xattrmap", ":map::user.virtiofs.:")
	args = append(args, "--socket-path", i.SocketPath)
	if len(i.SocketGroup) > 0 {
		args = append(args, "--socket-group", i.SocketGroup)

	}

	return args
}

func (v VirtiofsConfig) String() string { return joinArgs(v.CommandArgs()) }
