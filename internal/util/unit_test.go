package util

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadIECSize(t *testing.T) {
	tests := []struct {
		input string
		want  IECSize
	}{
		{input: "1126", want: 1126},
		{input: "1K", want: 1024},
		{input: "1Ki", want: 1024},
		{input: "1.5K", want: 1536},
		{input: "4G", want: 4 * 1024 * 1024 * 1024},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := LoadIECSize(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("LoadIECSize(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadIECSizeInvalidInput(t *testing.T) {
	for _, input := range []string{"", "abc", "1W"} {
		t.Run(input, func(t *testing.T) {
			if _, err := LoadIECSize(input); err == nil {
				t.Fatalf("LoadIECSize(%q) error = nil, want error", input)
			}
		})
	}
}

func TestIECSizeTextAndString(t *testing.T) {
	size := IECSize(4 * 1024 * 1024 * 1024)
	if got, want := size.String(), "4G"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	text, err := size.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(text), "4G"; got != want {
		t.Fatalf("MarshalText() = %q, want %q", got, want)
	}
	var parsed IECSize
	if err := parsed.UnmarshalText([]byte("4G")); err != nil {
		t.Fatal(err)
	}
	if parsed != size {
		t.Fatalf("UnmarshalText() = %d, want %d", parsed, size)
	}
}

func TestIECSizeJSONYAMLRoundTrip(t *testing.T) {
	type wrapper struct {
		SZ IECSize `json:"sz" yaml:"sz"`
	}
	want := wrapper{SZ: IECSize(1126)}

	jsonBytes, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if got, wantJSON := string(jsonBytes), `{"sz":"1.1K"}`; got != wantJSON {
		t.Fatalf("json = %s, want %s", got, wantJSON)
	}
	var gotJSON wrapper
	if err := json.Unmarshal(jsonBytes, &gotJSON); err != nil {
		t.Fatal(err)
	}
	if gotJSON.SZ != want.SZ {
		t.Fatalf("json roundtrip = %d, want %d", gotJSON.SZ, want.SZ)
	}

	yamlBytes, err := yaml.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if got, wantYAML := string(yamlBytes), "sz: 1.1K\n"; got != wantYAML {
		t.Fatalf("yaml = %q, want %q", got, wantYAML)
	}
	var gotYAML wrapper
	if err := yaml.Unmarshal(yamlBytes, &gotYAML); err != nil {
		t.Fatal(err)
	}
	if gotYAML.SZ != want.SZ {
		t.Fatalf("yaml roundtrip = %d, want %d", gotYAML.SZ, want.SZ)
	}
}
