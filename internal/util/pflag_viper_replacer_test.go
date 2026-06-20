package util

import (
	"strings"
	"testing"

	flags "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func TestPFlagViperReplacerVisitAllUsesReplacedNames(t *testing.T) {
	flagSet := flags.NewFlagSet("test", flags.ContinueOnError)
	flagSet.String("virtiofs-socket-filename-template", "virtiofs.sock", "")

	var names []string
	PFlagViperReplacer{
		FlagSet:  flagSet,
		Replacer: strings.NewReplacer("-", ".", "_", "."),
	}.VisitAll(func(flag viper.FlagValue) {
		names = append(names, flag.Name())
	})

	if len(names) != 1 || names[0] != "virtiofs.socket.filename.template" {
		t.Fatalf("VisitAll() names = %v, want replaced dotted key", names)
	}
}

func TestPFlagViperReplacerBindsFlagsAndEnv(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	flagSet := flags.NewFlagSet("test", flags.ContinueOnError)
	flagSet.String("cloudhypervisor-path", "/usr/bin/cloud-hypervisor", "")
	flagSet.String("virtiofs-socket-filename-template", "virtiofs.sock", "")
	if err := flagSet.Parse([]string{"--cloudhypervisor-path=/bin/sh"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	t.Setenv("VIRTIOFS_SOCKET_FILENAME_TEMPLATE", "env.sock")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()

	if err := viper.BindFlagValues(PFlagViperReplacer{
		FlagSet:  flagSet,
		Replacer: strings.NewReplacer("-", ".", "_", "."),
	}); err != nil {
		t.Fatalf("BindFlagValues() error = %v", err)
	}

	if got := viper.GetString("cloudhypervisor.path"); got != "/bin/sh" {
		t.Fatalf("cloudhypervisor.path = %q, want %q", got, "/bin/sh")
	}
	if got := viper.GetString("virtiofs.socket.filename.template"); got != "env.sock" {
		t.Fatalf("virtiofs.socket.filename.template = %q, want %q", got, "env.sock")
	}
}
