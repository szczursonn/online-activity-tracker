package webapi

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

type PlayerSummary struct {
	PersonaName  string
	ProfileURL   string
	AvatarURL    string
	PersonaState int
	GameID       uint64
}

var escapedComma = url.QueryEscape(",")

func (c *Client) FetchPlayerSummaries(ctx context.Context, steamIDs []uint64) (map[uint64]*PlayerSummary, error) {
	u := c.baseURL
	u.Path += "/ISteamUser/GetPlayerSummaries/v0002"
	var querySb strings.Builder
	querySb.WriteString(u.RawQuery)
	querySb.WriteString("&format=json&steamids=")
	for i, steamID := range steamIDs {
		if i > 0 {
			querySb.WriteString(escapedComma)
		}
		querySb.WriteString(strconv.FormatUint(steamID, 10))
	}
	u.RawQuery = querySb.String()

	res := struct {
		Response struct {
			Players []*struct {
				SteamID                  string `json:"steamid"`
				PersonaName              string `json:"personaname"`
				ProfileURL               string `json:"profileurl"`
				AvatarFullURL            string `json:"avatarfull"`
				PersonaState             int    `json:"personastate"`
				CommunityVisibilityState int    `json:"communityvisibilitystate"`
				GameID                   string `json:"gameid"`
			} `json:"players"`
		} `json:"response"`
	}{}
	if err := c.callJSONAPI(ctx, &u, &res); err != nil {
		return nil, err
	}

	summaries := make(map[uint64]*PlayerSummary, len(res.Response.Players))
	for _, player := range res.Response.Players {
		const communityVisibilityStatePublic = 3
		if player == nil || player.CommunityVisibilityState != communityVisibilityStatePublic {
			continue
		}

		steamID, err := strconv.ParseUint(player.SteamID, 10, 64)
		if err != nil {
			continue
		}

		ps := &PlayerSummary{
			PersonaName:  strings.TrimSpace(player.PersonaName),
			ProfileURL:   player.ProfileURL,
			AvatarURL:    player.AvatarFullURL,
			PersonaState: player.PersonaState,
		}
		ps.GameID, _ = strconv.ParseUint(player.GameID, 10, 64)

		summaries[steamID] = ps
	}

	return summaries, nil
}
