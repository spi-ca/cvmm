package hvm

import (
	"context"
	"errors"
	"testing"

	"amuz.es/src/spi-ca/chmgr/internal/model"
)

func TestVirtiofsConfigUnit(t *testing.T) {
	ctx := context.Background()
	var i virtiofsdJoiner

	cfgs := make([]model.VirtiofsConfig, 10)
	selectorErrorChan := i.Execute(ctx, cfgs)

	var errs []error

	for selectorErr := range selectorErrorChan {
		errs = append(errs, selectorErr)
	}
	err := errors.Join(errs...)
	if err != nil {
		panic(err)
	}
}
