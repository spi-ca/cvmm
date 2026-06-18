package sys

import (
	"fmt"
	"os/user"
	"strconv"
	"syscall"
)

// LookupUid resolves a user name to a numeric uid.
func LookupUid(name string) (uint32, error) {
	u, err := user.Lookup(name)
	if err != nil {
		return 0, fmt.Errorf("failed to find user %s : %w", name, err)
	}

	return convertId(u.Uid)
}

// LookupGid resolves a group name to a numeric gid.
func LookupGid(name string) (uint32, error) {
	u, err := user.LookupGroup(name)
	if err != nil {
		return 0, fmt.Errorf("failed to find group %s : %w", name, err)
	}

	return convertId(u.Gid)
}

// LookupSupplimentaryGroups resolves supplementary group ids for a user.
func LookupSupplimentaryGroups(name string) ([]uint32, error) {
	u, err := user.Lookup(name)
	if err != nil {
		return nil, err
	}

	sgidMap := map[uint32]bool{}

	gid, err := convertId(u.Gid)
	if err != nil {
		return nil, err
	}

	sgids, err := u.GroupIds()
	if err != nil {
		return nil, err
	}

	ret := make([]uint32, 0, len(sgidMap)+1)

	ret = append(ret, gid)

	for _, sgid := range sgids {
		sg, err := convertId(sgid)
		if err != nil {
			return nil, err
		} else if sg == gid {
			continue
		} else {
			ret = append(ret, sg)
		}
	}

	return ret, nil
}

// LookupUserName resolves a numeric uid and returns the canonical uid string from the host user database.
func LookupUserName(uid uint32) (string, error) {
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return "", fmt.Errorf("failed to find user %d : %w", uid, err)
	}
	return u.Uid, nil
}

// LookupGroupName resolves a numeric gid to a group name.
func LookupGroupName(gid uint32) (string, error) {
	g, err := user.LookupGroupId(strconv.FormatUint(uint64(gid), 10))
	if err != nil {
		return "", fmt.Errorf("failed to find group %d : %w", gid, err)
	}
	return g.Name, nil
}

// LookupCredentials resolves a user name into uid, primary gid, and supplementary groups for child process credentials.
func LookupCredentials(name string) (*syscall.Credential, error) {
	u, err := user.Lookup(name)
	if err != nil {
		return nil, err
	}

	uid, err := convertId(u.Uid)
	if err != nil {
		return nil, err
	}

	sgidMap := map[uint32]bool{}

	gid, err := convertId(u.Gid)
	if err != nil {
		return nil, err
	}

	sgidStrings, err := u.GroupIds()
	if err != nil {
		return nil, err
	}

	sgids := make([]uint32, 0, len(sgidMap)+1)

	sgids = append(sgids, gid)

	for _, sgid := range sgidStrings {
		sg, err := convertId(sgid)
		if err != nil {
			return nil, err
		} else if sg == gid {
			continue
		} else {
			sgids = append(sgids, sg)
		}
	}

	return &syscall.Credential{
		Uid:    uid,
		Gid:    gid,
		Groups: sgids,
	}, nil
}

// convertId converts lookup command output into an integer id.
func convertId(id string) (uint32, error) {
	parsed, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return 0, err
	}

	return uint32(parsed), nil
}
