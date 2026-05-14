package storefrontapi

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

type AppDetails struct {
	ID             uint64
	Name           string
	HeaderImageURL string
}

func (c *Client) FetchAppDetails(ctx context.Context, appID uint64) (*AppDetails, error) {
	appIDStr := fmt.Sprint(appID)
	u := baseURL
	u.Path += "/appdetails"
	u.RawQuery = fmt.Sprintf("appids=%s&filters=basic", appIDStr)

	res := map[string]struct {
		Data struct {
			Name        string `json:"name"`
			HeaderImage string `json:"header_image"`
		} `json:"data"`
	}{}
	if err := c.callAPI(ctx, &u, &res); err != nil {
		return nil, err
	}

	entry, ok := res[appIDStr]
	if !ok {
		return nil, fmt.Errorf(errPrefix + "missing app id key in app details response")
	}

	ad := &AppDetails{
		ID:   appID,
		Name: strings.TrimSpace(entry.Data.Name),
	}

	if parsed, err := url.ParseRequestURI(entry.Data.HeaderImage); err == nil {
		ad.HeaderImageURL = parsed.String()
	}

	return ad, nil
}
