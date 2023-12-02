package sys

import (
	"fmt"
	"os/user"
	"strconv"
)

func LookupUid(name string) (uint32, error) {
	u, err := user.Lookup(name)
	if err != nil {
		return 0, fmt.Errorf("failed to find user %s : %w", name, err)
	}
	id, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid uid format for user %s : %w", u.Uid, name, err)
	}
	return uint32(id), nil
}

func LookupGid(name string) (uint32, error) {
	u, err := user.LookupGroup(name)
	if err != nil {
		return 0, fmt.Errorf("failed to find group %s : %w", name, err)
	}
	id, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid gid format for user %s : %w", u.Gid, name, err)
	}
	return uint32(id), nil
}
