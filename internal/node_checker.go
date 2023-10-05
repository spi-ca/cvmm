package internal

import (
	"amuz.es/src/spi-ca/chmgr/internal/hvm"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

func NodeStatusChecker(ctx context.Context, client *http.Client, expectedStatus hvm.NodeStatus, errorChan chan<- error) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("panic on nodeStatusChecker: %v", err)
		}
		close(errorChan)
	}()

	i := &struct {
		Status hvm.NodeStatus `json:"state"`
	}{}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		resp, err := client.Get("http://localhost/api/v1/vm.info")
		if err != nil {
			errorChan <- err
			return
		} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errorChan <- fmt.Errorf("http error(%d): %s", resp.StatusCode, http.StatusText(resp.StatusCode))
			return
		}

		d := json.NewDecoder(resp.Body)
		err = d.Decode(&i)
		if err != nil {
			errorChan <- err
			return
		} else if i.Status != expectedStatus {
			errorChan <- fmt.Errorf("failed status %s, expected: %s", i.Status, expectedStatus)
			return
		} else {
			_ = <-ticker.C
		}
	}
}
