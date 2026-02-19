package zen

import (
	"context"
	"net/http"
	"time"
)

// defaultResponseHeaderTimeout is applied to the transport of the internally
// created HTTP client when the caller does not supply their own *http.Client.
// It guards against a server that accepts the TCP connection but never sends
// response headers (hung server / network black-hole).  It deliberately does
// NOT use http.Client.Timeout, which is a total round-trip deadline that fires
// while the response body is still being read and therefore kills long-running
// SSE streams.
const defaultResponseHeaderTimeout = 30 * time.Second

type Client struct {
	cfg        Config
	httpClient *http.Client
}

func NewClient(cfg Config) (*Client, error) {
	if err := cfg.applyDefaults(); err != nil {
		return nil, err
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		// Clone the default transport so we can set ResponseHeaderTimeout
		// without mutating the global.  Use a comma-ok assertion so we don't
		// panic if http.DefaultTransport has been replaced (e.g. by test
		// helpers or instrumentation libraries such as go-vcr / otelhttp).
		var transport *http.Transport
		if t, ok := http.DefaultTransport.(*http.Transport); ok {
			transport = t.Clone()
		} else {
			transport = &http.Transport{}
		}
		transport.ResponseHeaderTimeout = defaultResponseHeaderTimeout
		httpClient = &http.Client{
			Transport: transport,
			// cfg.Timeout is 0 by default (unlimited).  If the caller sets it
			// explicitly they accept that it will terminate the body read —
			// document this clearly in Config.Timeout's godoc.
			Timeout: cfg.Timeout,
		}
	}

	return &Client{
		cfg:        cfg,
		httpClient: httpClient,
	}, nil
}

func (c *Client) Do(ctx context.Context, method, path string, body []byte) ([]byte, http.Header, error) {
	return c.doRequest(ctx, method, path, body)
}
