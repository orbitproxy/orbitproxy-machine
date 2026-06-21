package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/parnurzeal/gorequest"
)

const defaultHTTPTimeout = 15 * time.Second

var (
	once       sync.Once
	jsonClient *Client
	mu         sync.RWMutex
	override   *http.Client
)

type Client struct {
	timeout    time.Duration
	httpClient *http.Client
}

func Init() {
	once.Do(func() {
		jsonClient = New(defaultHTTPTimeout)
	})
}

func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	return &Client{timeout: timeout}
}

func JSON() *Client {
	Init()

	mu.RLock()
	o := override
	base := jsonClient
	mu.RUnlock()

	if o != nil {
		return base.WithHTTPClient(o)
	}
	return base
}

func SetOverride(httpClient *http.Client) {
	mu.Lock()
	defer mu.Unlock()
	override = httpClient
}

func (c *Client) WithHTTPClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		return c
	}
	clone := *c
	clone.httpClient = httpClient
	return &clone
}

func PostJSON[Req, Resp any](c *Client, ctx context.Context, url string, reqBody Req) (Resp, int, error) {
	var zeroResp Resp

	agent := gorequest.New().Timeout(c.timeout).Type("json")
	if c.httpClient != nil {
		agent.Client = c.httpClient
	}
	if ctx != nil {
		agent = agent.Context(ctx)
	}

	var respBody Resp
	httpResp, _, errs := agent.Post(url).
		Set("Content-Type", "application/json").
		SendStruct(reqBody).
		EndStruct(&respBody)
	if err := firstError(append(errs, agent.Errors...)); err != nil {
		return zeroResp, 0, err
	}
	if httpResp == nil {
		return zeroResp, 0, fmt.Errorf("http request returned no response")
	}
	return respBody, httpResp.StatusCode, nil
}

func firstError(errs []error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
