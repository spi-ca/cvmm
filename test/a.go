package main

import (
	"log"

	"amuz.es/src/spi-ca/cvmm/internal/util/sys"
)

func main() {
	sg, err := sys.LookupSupplimentaryGroups("hvm")
	if err != nil {
		panic(err)
	}
	log.Printf("sgs : %v", sg)
}
