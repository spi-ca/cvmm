package util

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMACAddressTextAndString(t *testing.T) {
	addr := MustLoadMACAddress("52:54:00:02:38:f0")
	if got, want := addr.String(), "52:54:00:02:38:f0"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	text, err := addr.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(text), "52:54:00:02:38:f0"; got != want {
		t.Fatalf("MarshalText() = %q, want %q", got, want)
	}

	var parsed MACAddress
	if err := parsed.UnmarshalText([]byte("52:54:00:02:38:f0")); err != nil {
		t.Fatal(err)
	}
	if got, want := parsed.String(), addr.String(); got != want {
		t.Fatalf("UnmarshalText() = %q, want %q", got, want)
	}
}

func TestMACAddressJSONYAMLRoundTrip(t *testing.T) {
	type wrapper struct {
		SZ MACAddress `json:"sz" yaml:"sz"`
	}
	want := wrapper{SZ: MustLoadMACAddress("01:02:03:04:05:06")}

	jsonBytes, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if got, wantJSON := string(jsonBytes), `{"sz":"01:02:03:04:05:06"}`; got != wantJSON {
		t.Fatalf("json = %s, want %s", got, wantJSON)
	}
	var gotJSON wrapper
	if err := json.Unmarshal(jsonBytes, &gotJSON); err != nil {
		t.Fatal(err)
	}
	if gotJSON.SZ.String() != want.SZ.String() {
		t.Fatalf("json roundtrip = %q, want %q", gotJSON.SZ, want.SZ)
	}

	yamlBytes, err := yaml.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if got, wantYAML := string(yamlBytes), "sz: \"01:02:03:04:05:06\"\n"; got != wantYAML {
		t.Fatalf("yaml = %q, want %q", got, wantYAML)
	}
	var gotYAML wrapper
	if err := yaml.Unmarshal(yamlBytes, &gotYAML); err != nil {
		t.Fatal(err)
	}
	if gotYAML.SZ.String() != want.SZ.String() {
		t.Fatalf("yaml roundtrip = %q, want %q", gotYAML.SZ, want.SZ)
	}
}

func TestMACAddressInvalidInput(t *testing.T) {
	if _, err := LoadMACAddress("not-a-mac"); err == nil {
		t.Fatal("LoadMACAddress() error = nil, want error")
	}
	var addr MACAddress
	if err := addr.UnmarshalText([]byte("not-a-mac")); err == nil {
		t.Fatal("UnmarshalText() error = nil, want error")
	}
}

func TestMACAddressGenerateIfName(t *testing.T) {
	addr := MustLoadMACAddress("52:54:00:02:38:f0")
	if got, want := addr.GenerateIfName("vmtap-"), "vmtap-0238f0"; got != want {
		t.Fatalf("GenerateIfName() = %q, want %q", got, want)
	}
}

func TestGenerateKvmMACAddress(t *testing.T) {
	addr := GenerateKvmMACAddress()
	if len(addr) != 6 {
		t.Fatalf("len(GenerateKvmMACAddress()) = %d, want 6", len(addr))
	}
	if got, want := addr[:3].String(), "52:54:00"; got != want {
		t.Fatalf("GenerateKvmMACAddress prefix = %q, want %q", got, want)
	}
}
