package sys

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkAcquirePidFileReplacing(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.pid")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cleanup, err := AcquirePidFileReplacing(path, os.Getpid())
		if err != nil {
			b.Fatalf("AcquirePidFileReplacing() error = %v", err)
		}
		cleanup()
	}
}

func BenchmarkReadPidFile(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.pid")
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pid, err := ReadPidFile(path)
		if err != nil {
			b.Fatalf("ReadPidFile() error = %v", err)
		}
		if pid != os.Getpid() {
			b.Fatalf("pid = %d, want %d", pid, os.Getpid())
		}
	}
}

func BenchmarkIsPidFileActive(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.pid")
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !IsPidFileActive(path) {
			b.Fatal("IsPidFileActive() = false, want true")
		}
	}
}
