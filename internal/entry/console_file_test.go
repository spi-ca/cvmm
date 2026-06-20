package entry

import "testing"

func TestConsolePtyPathBuildsDevPtsPath(t *testing.T) {
	if got, want := consolePtyPath(42), "/dev/pts/42"; got != want {
		t.Fatalf("consolePtyPath() = %q, want %q", got, want)
	}
}
