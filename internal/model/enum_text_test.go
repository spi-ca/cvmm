package model

import "testing"

func TestEnumMarshalTextZeroValueReturnsEmpty(t *testing.T) {
	tests := []struct {
		name string
		text func() ([]byte, error)
	}{
		{name: "ConsoleMode", text: ConsoleModeInvalid.MarshalText},
		{name: "DebugConsoleMode", text: DebugConsoleModeInvalid.MarshalText},
		{name: "NodeStatus", text: NodeStatusInvalid.MarshalText},
		{name: "LandlockMode", text: LandlockModeInvalid.MarshalText},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.text()
			if err != nil {
				t.Fatalf("MarshalText() error = %v", err)
			}
			if string(got) != "" {
				t.Fatalf("MarshalText() = %q, want empty", string(got))
			}
		})
	}
}

func TestEnumUnmarshalTextRejectsInvalidInputWithoutMutation(t *testing.T) {
	consoleMode := ConsoleModePty
	if err := consoleMode.UnmarshalText([]byte("not-a-console-mode")); err == nil {
		t.Fatal("ConsoleMode.UnmarshalText() error = nil, want error")
	} else if consoleMode != ConsoleModePty {
		t.Fatalf("ConsoleMode mutated to %q on error", consoleMode)
	}

	debugConsoleMode := DebugConsoleModePty
	if err := debugConsoleMode.UnmarshalText([]byte("not-a-debug-console-mode")); err == nil {
		t.Fatal("DebugConsoleMode.UnmarshalText() error = nil, want error")
	} else if debugConsoleMode != DebugConsoleModePty {
		t.Fatalf("DebugConsoleMode mutated to %q on error", debugConsoleMode)
	}

	nodeStatus := NodeStatusRunning
	if err := nodeStatus.UnmarshalText([]byte("not-a-node-status")); err == nil {
		t.Fatal("NodeStatus.UnmarshalText() error = nil, want error")
	} else if nodeStatus != NodeStatusRunning {
		t.Fatalf("NodeStatus mutated to %q on error", nodeStatus)
	}

	landlockMode := LandlockModeReadWrite
	if err := landlockMode.UnmarshalText([]byte("not-a-landlock-mode")); err == nil {
		t.Fatal("LandlockMode.UnmarshalText() error = nil, want error")
	} else if landlockMode != LandlockModeReadWrite {
		t.Fatalf("LandlockMode mutated to %q on error", landlockMode)
	}
}
