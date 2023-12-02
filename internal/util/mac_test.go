package util

import (
	"encoding/hex"
	"encoding/json"
	"log"
	"testing"

	"gopkg.in/yaml.v3"
)

func Test_MACAddress_JSON(t *testing.T) {
	raw, _ := hex.DecodeString("010203040506")
	o := struct {
		SZ MACAddress
	}{SZ: MACAddress(raw)}

	marshalled, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	log.Printf("-> %s", marshalled)

	o2 := struct {
		SZ MACAddress
	}{}

	err = json.Unmarshal(marshalled, &o2)
	if err != nil {
		panic(err)
	}

	log.Printf("-> %d", o2.SZ)
}

func Test_MACAddress_YAML(t *testing.T) {
	raw, _ := hex.DecodeString("010203040506")
	o := struct {
		SZ MACAddress
	}{SZ: MACAddress(raw)}

	marshalled, err := yaml.Marshal(o)
	if err != nil {
		panic(err)
	}
	log.Printf("-> %s", marshalled)

	o2 := struct {
		SZ MACAddress
	}{}

	err = yaml.Unmarshal(marshalled, &o2)
	if err != nil {
		panic(err)
	}

	log.Printf("-> %d", o2.SZ)
}
func Test_MACAddress_Rand(t *testing.T) {
	chksum := [3]byte{
		0x02,
		0x38,
		0xf0,
	}
	addr := MACAddress{
		52, 54, 00, chksum[0], chksum[1], chksum[2],
	}

	log.Printf("-> %s", addr)
}
