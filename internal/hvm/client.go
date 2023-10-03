package hvm

import (
	"context"
	"errors"
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

func (c *Client) dialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", c.socketPath)
}

func (c *Client) checkRedirect(_ *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return nil
}
