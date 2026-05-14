package storefrontapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const errPrefix = "steam/storefrontapi: "

var baseURL = url.URL{
	Scheme: "https",
	Host:   "store.steampowered.com",
	Path:   "/api",
}

type ClientOptions struct {
	HTTPClient *http.Client
}

func (opts *ClientOptions) applyDefaults() {
	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}
}

type Client struct {
	httpClient *http.Client
}

func NewClient(opts *ClientOptions) *Client {
	opts.applyDefaults()

	return &Client{
		httpClient: opts.HTTPClient,
	}
}

func (c *Client) callAPI(ctx context.Context, u *url.URL, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf(errPrefix+"creating request (%s): %w", u, err)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf(errPrefix+"doing request (%s): %w", u, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf(errPrefix+"unexpected response status code (%s): %d", u, res.StatusCode)
	}

	if err := json.NewDecoder(res.Body).Decode(v); err != nil {
		return fmt.Errorf(errPrefix+"unmarshaling response body (%s): %w", u, err)
	}

	return nil
}
