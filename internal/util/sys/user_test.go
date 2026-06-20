package sys

import "testing"

func TestLookupUserGroupErrorPaths(t *testing.T) {
	const missingUser = "cvmm-test-user-does-not-exist"
	const missingGroup = "cvmm-test-group-does-not-exist"
	const missingID = ^uint32(0)

	if _, err := LookupUid(missingUser); err == nil {
		t.Fatal("LookupUid() error = nil, want missing-user error")
	}
	if _, err := LookupGid(missingGroup); err == nil {
		t.Fatal("LookupGid() error = nil, want missing-group error")
	}
	if _, err := LookupSupplimentaryGroups(missingUser); err == nil {
		t.Fatal("LookupSupplimentaryGroups() error = nil, want missing-user error")
	}
	if _, err := LookupUserName(missingID); err == nil {
		t.Fatal("LookupUserName() error = nil, want missing-id error")
	}
	if _, err := LookupGroupName(missingID); err == nil {
		t.Fatal("LookupGroupName() error = nil, want missing-id error")
	}
	if _, err := LookupCredentials(missingUser); err == nil {
		t.Fatal("LookupCredentials() error = nil, want missing-user error")
	}
}
