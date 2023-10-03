package util

import (
	"encoding/json"
	"log"
	"testing"

	"gopkg.in/yaml.v3"
)

func Test_IECSize_JSON(t *testing.T) {
	o := struct {
		SZ IECSize
	}{SZ: 1126}

	marshalled, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	log.Printf("-> %s", marshalled)

	o2 := struct {
		SZ IECSize
	}{}

	err = json.Unmarshal(marshalled, &o2)
	if err != nil {
		panic(err)
	}

	log.Printf("-> %d", o2.SZ)
}

func Test_IECSize_YAML(t *testing.T) {
	o := struct {
		SZ IECSize
	}{SZ: 1126}

	marshalled, err := yaml.Marshal(o)
	if err != nil {
		panic(err)
	}
	log.Printf("-> %s", marshalled)

	o2 := struct {
		SZ IECSize
	}{}

	err = yaml.Unmarshal(marshalled, &o2)
	if err != nil {
		panic(err)
	}

	log.Printf("-> %d", o2.SZ)
}
