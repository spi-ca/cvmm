package util

import (
	"strings"
	"text/template"
)

type (
	Format     template.Template
	FormatArgs map[string]any
)

func FormatStr(text string) (*Format, error) {
	tmpl, err := template.New("").Parse(text)
	return (*Format)(tmpl), err
}

func F(text string) *Format {
	tmpl, err := FormatStr(text)
	if err != nil {
		panic(err)
	}
	return tmpl
}

func (tmpl *Format) Render(args FormatArgs) (string, error) {
	output := new(strings.Builder)
	err := (*template.Template)(tmpl).Execute(output, args)
	return output.String(), err
}

func (tmpl *Format) R(args FormatArgs) string {
	s, err := tmpl.Render(args)
	if err != nil {
		panic(err)
	}
	return s
}
