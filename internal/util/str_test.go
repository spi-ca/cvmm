package util

import "testing"

func TestFormatRender(t *testing.T) {
	tmpl, err := FormatStr("{{.directoryName}}.yaml")
	if err != nil {
		t.Fatal(err)
	}
	got, err := tmpl.Render(FormatArgs{"directoryName": "ab"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "ab.yaml"; got != want {
		t.Fatalf("Render() = %q, want %q", got, want)
	}
	if got := tmpl.R(FormatArgs{"directoryName": "cd"}); got != "cd.yaml" {
		t.Fatalf("R() = %q, want %q", got, "cd.yaml")
	}
}

func TestFormatStrInvalidTemplate(t *testing.T) {
	if _, err := FormatStr("{{"); err == nil {
		t.Fatal("FormatStr() error = nil, want error")
	}
}

func TestAppendFileSuffix(t *testing.T) {
	tests := []struct {
		name   string
		tmpl   string
		suffix string
		want   string
	}{
		{name: "extension", tmpl: "virtiofs.sock", suffix: "configuration", want: "virtiofs_configuration.sock"},
		{name: "path", tmpl: "/run/virtiofs.sock", suffix: "cfg", want: "/run/virtiofs_cfg.sock"},
		{name: "no extension", tmpl: "virtiofs", suffix: "cfg", want: "virtiofs_cfg"},
		{name: "empty prefix", tmpl: ".sock", suffix: "cfg", want: "cfg.sock"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AppendFileSuffix(tt.tmpl, tt.suffix); got != tt.want {
				t.Fatalf("AppendFileSuffix(%q, %q) = %q, want %q", tt.tmpl, tt.suffix, got, tt.want)
			}
		})
	}
}
