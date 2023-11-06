package model

import (
	"strconv"
)

type VirtiofsConfig struct {
	Directory      string
	SocketPath     string
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
	args = append(args, "--socket-path", i.SocketPath)
	return args
}

func (v VirtiofsConfig) String() string { return joinArgs(v.CommandArgs()) }
