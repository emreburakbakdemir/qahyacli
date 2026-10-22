package jobapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Client is what `qahyacli env ...` uses inside a step.
type Client struct {
	http *http.Client 
	step string
}

func NewClientFromEnv() (*Client, error) {
	sock := os.Getenv(SocketEnv)
	if sock == "" {
		return nil, fmt.Errorf("%s is not set: `qahyacli env` only works inside a step started by `qahyacli run`", SocketEnv)
	}

	// a normal http.Client, except every connection goes to the socket.
	// the host in request URLs is ignored, so we use a placeholder.
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", sock)
		},
	}
	return &Client{
		http: &http.Client{Transport: tr, Timeout: 10 * time.Second},
		step: os.Getenv(StepEnv),
	}, nil
}

func (c *Client) List() ([]Entry, error) {
	var out []Entry
	err := c.do(http.MethodGet, "/env", nil, &out)
	return out, err
}

func (c *Client) Get(name string) (Entry, error) {
	var out Entry
	err := c.do(http.MethodGet, "/env/"+url.PathEscape(name), nil, &out)
	return out, err
}

func (c *Client) Set(name, value string) error {
	return c.do(http.MethodPut, "/env/"+url.PathEscape(name), setRequest{Value: value}, nil)
}

func (c *Client) Unset(name string) error {
	return c.do(http.MethodDelete, "/env/"+url.PathEscape(name), nil, nil)
}

func (c *Client) Load(path, profile string, export bool) ([]Entry, error) {
	var out loadResponse
	err := c.do(http.MethodPost, "/load", loadRequest{Path: path, Profile: profile, Export: export}, &out)
	return out.Entries, err
}

func (c *Client) do(method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, "http://qahya"+path, body)
	if err != nil {
		return err
	}
	req.Header.Set(stepHeader, c.step)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("talk to job api: %w", err)
	}
	defer resp.Body.Close()

	// surface the server's message: a bare "exit 1" from a step is miserable to debug
	if resp.StatusCode >= 400 {
		var e errorResponse
		if json.NewDecoder(resp.Body).Decode(&e) == nil && e.Error != "" {
			return errors.New(e.Error)
		}
		return fmt.Errorf("job api: %s", resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
