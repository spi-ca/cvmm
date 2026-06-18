package hvm

import "testing"

func TestClientActions(t *testing.T) {
	want := []ClientAction{
		ClientActionVmmPing,
		ClientActionVmmShutdown,
		ClientActionVmmNmi,
		ClientActionVmInfo,
		ClientActionVmCounters,
		ClientActionVmCreate,
		ClientActionVmDelete,
		ClientActionVmBoot,
		ClientActionVmPause,
		ClientActionVmResume,
		ClientActionVmShutdown,
		ClientActionVmReboot,
		ClientActionVmPowerButton,
		ClientActionVmResize,
		ClientActionVmResizeZone,
		ClientActionVmAddDevice,
		ClientActionVmAddUserDevice,
		ClientActionVmRemoveDevice,
		ClientActionVmAddDisk,
		ClientActionVmAddFs,
		ClientActionVmAddPmem,
		ClientActionVmAddNet,
		ClientActionVmAddVsock,
		ClientActionVmAddVdpa,
		ClientActionVmSnapshot,
		ClientActionVmCoredump,
		ClientActionVmRestore,
		ClientActionVmReceiveMigration,
		ClientActionVmSendMigration,
	}
	got := ClientActions()
	if len(got) != len(want) {
		t.Fatalf("len(ClientActions()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ClientActions()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestClientActionNameOfRoundTrip(t *testing.T) {
	for _, action := range ClientActions() {
		got, err := ClientActionNameOf(action.String())
		if err != nil {
			t.Fatalf("ClientActionNameOf(%q) returned error: %v", action.String(), err)
		}
		if got != action {
			t.Fatalf("ClientActionNameOf(%q) = %q, want %q", action.String(), got, action)
		}
	}
}

func TestClientActionUnmarshalTextRoundTrip(t *testing.T) {
	for _, action := range ClientActions() {
		var got ClientAction
		if err := got.UnmarshalText([]byte(action.String())); err != nil {
			t.Fatalf("UnmarshalText(%q) returned error: %v", action.String(), err)
		}
		if got != action {
			t.Fatalf("UnmarshalText(%q) = %q, want %q", action.String(), got, action)
		}
	}
}

func TestClientActionNameOfInvalid(t *testing.T) {
	got, err := ClientActionNameOf("not-a-client-action")
	if err == nil {
		t.Fatal("ClientActionNameOf() error = nil, want error")
	}
	if got != ClientActionInvalid {
		t.Fatalf("ClientActionNameOf() = %q, want %q", got, ClientActionInvalid)
	}
}

func TestClientActionUnmarshalTextInvalid(t *testing.T) {
	action := ClientActionVmInfo
	if err := action.UnmarshalText([]byte("not-a-client-action")); err == nil {
		t.Fatal("UnmarshalText() error = nil, want error")
	}
	if action != ClientActionVmInfo {
		t.Fatalf("UnmarshalText() mutated value to %q on error", action)
	}
}

func TestClientActionStringInvalid(t *testing.T) {
	if got := ClientActionInvalid.String(); got != "" {
		t.Fatalf("ClientActionInvalid.String() = %q, want empty string", got)
	}
}
