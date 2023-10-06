package hvm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

type Client struct {
	cli        http.Client
	socketPath string
}

func NewClient(socketPath string) *Client {
	c := Client{
		socketPath: socketPath,
	}

	c.cli.Transport = &http.Transport{
		DialContext: c.dialContext,
	}
	c.cli.CheckRedirect = c.checkRedirect
	c.cli.Timeout = 5 * time.Second

	return &c
}

func (c *Client) Close() { c.cli.CloseIdleConnections() }

func (c *Client) Info() error {
	resp, err := c.cli.Get("http://localhost/api/v1/vm.info")
	if err != nil {
		return err
	} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http error(%d): %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	//todo todo
	return nil
}
func (c *Client) dialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", c.socketPath)
}

func (c *Client) checkRedirect(_ *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return nil
}
