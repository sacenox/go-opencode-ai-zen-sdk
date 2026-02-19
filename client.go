package zen

import "net/http"

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
		transport.ResponseHeaderTimeout = cfg.ResponseHeaderTimeout
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
