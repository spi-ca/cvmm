package util

import (
	"path/filepath"
	"strings"
	"text/template"
)

type (
	// Format wraps a parsed text/template used by cvmm log and usage rendering helpers.
	Format template.Template
	// FormatArgs supplies named values to a Format template.
	FormatArgs map[string]any
)

// FormatStr renders a Go template string with the supplied arguments.
func FormatStr(text string) (*Format, error) {
	tmpl, err := template.New("").Parse(text)
	return (*Format)(tmpl), err
}

// F converts a template string into a reusable Format helper.
func F(text string) *Format {
	tmpl, err := FormatStr(text)
	if err != nil {
		panic(err)
	}
	return tmpl
}

// Render executes the template with the supplied arguments.
func (tmpl *Format) Render(args FormatArgs) (string, error) {
	output := new(strings.Builder)
	err := (*template.Template)(tmpl).Execute(output, args)
	return output.String(), err
}

// R is a short alias for Render.
func (tmpl *Format) R(args FormatArgs) string {
	s, err := tmpl.Render(args)
	if err != nil {
		panic(err)
	}
	return s
}

// AppendFileSuffix inserts a suffix before a filename extension.
func AppendFileSuffix(tmpl, suffix string) string {
	ext := filepath.Ext(tmpl)
	prefix := tmpl[:len(tmpl)-len(ext)]

	var b strings.Builder
	b.WriteString(prefix)

	if len(prefix) > 0 {
		b.WriteByte('_')
	}

	b.WriteString(suffix)
	b.WriteString(ext)

	return b.String()
}
