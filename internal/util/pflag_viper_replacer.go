package util

import (
	flags "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"strings"
)

// pflagValue is a wrapper around *flags.Flag that implements viper.FlagValue.
type pflagViperValue struct {
	flag *flags.Flag
	name string
}

// HasChanged returns whether the flag has changes or not.
func (p pflagViperValue) HasChanged() bool {
	return p.flag.Changed
}

// Name returns the Viper key name for the wrapped flag.
func (p pflagViperValue) Name() string {
	if len(p.name) > 0 {
		return p.name
	}
	return p.flag.Name
}

// ValueString returns the value of the flag as a string.
func (p pflagViperValue) ValueString() string {
	return p.flag.Value.String()
}

// ValueType returns the type of the flag as a string.
func (p pflagViperValue) ValueType() string {
	return p.flag.Value.Type()
}

// PFlagViperReplacer adapts pflag names to Viper key names during binding.
type PFlagViperReplacer struct {
	*flags.FlagSet
	Replacer *strings.Replacer
}

// VisitAll iterates over all *flags.Flag inside the *flags.FlagSet.
func (p PFlagViperReplacer) VisitAll(fn func(flag viper.FlagValue)) {
	p.FlagSet.VisitAll(func(flag *flags.Flag) {
		name := flag.Name
		if p.Replacer != nil {
			name = p.Replacer.Replace(name)
		}
		fn(pflagViperValue{flag: flag, name: name})
	})
}
