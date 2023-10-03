package util

import (
	"log"
	"strings"
	"testing"
	"text/template"
)

func Test_Tmpl(t *testing.T) {
	tmpl, err := template.New("").Parse("{{.directoryName}}.yaml")
	if err != nil {
		panic(err)
	}

	b := new(strings.Builder)
	err = tmpl.Execute(b, map[string]string{
		"directoryName": "ab",
	})
	if err != nil {
		panic(err)
	}
	log.Printf("-> %s", b.String())

}
