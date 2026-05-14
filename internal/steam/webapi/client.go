package webapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const errPrefix = "steam/webapi: "

type ClientOptions struct {
	HTTPClient *http.Client
	Key        string
}

func (opts *ClientOptions) applyDefaultsAndValidate() error {
	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}

	if opts.Key == "" {
		return fmt.Errorf(errPrefix + "missing api key")
	}

	return nil
}

type Client struct {
	httpClient *http.Client
	baseURL    url.URL
}

func NewClient(opts *ClientOptions) (*Client, error) {
	if err := opts.applyDefaultsAndValidate(); err != nil {
		return nil, err
	}

	return &Client{
		httpClient: opts.HTTPClient,
		baseURL: url.URL{
			Scheme:   "https",
			Host:     "api.steampowered.com",
			Path:     "/",
			RawQuery: fmt.Sprintf("key=%s", url.QueryEscape(opts.Key)),
		},
	}, nil
}

func (c *Client) callJSONAPI(ctx context.Context, u *url.URL, v any) error {
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
