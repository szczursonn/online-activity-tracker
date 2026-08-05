package viewer

import (
	"cmp"
	"fmt"
	"net/http"
	"strconv"

	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storediscord"
)

const (
	discordGuildsPath  = "/discord-guilds"
	discordGuildsTitle = "Discord Guilds"
)

var discordGuildsNavLink = navLink{
	Href:    discordGuildsPath,
	Label:   discordGuildsTitle,
	IconURL: discordLogoURL,
	IconAlt: "discord icon",
}

var discordGuildTemplate = mustParseTemplate(timelinePageTemplateFileName, "discord_guild.html")

func discordGuildImageAlt(name string) string {
	return fmt.Sprintf("discord guild %q icon", name)
}

func registerDiscordGuildsPage(s *Server, mux *http.ServeMux) {
	mux.Handle("GET "+discordGuildsPath, newPageHandler(s, cardGridTemplate, func(tx *db.Tx) (any, error) {
		guilds, err := storediscord.GetAllGuilds(tx)
		if err != nil {
			return nil, err
		}

		return &cardGridPageData{
			Title: discordGuildsTitle,
			Items: mapSeq(guilds, func(guild storediscord.GetAllGuildsGuild) cardGridPageDataItem {
				name := displayNameStrOrID(guild.Name, guild.ID)

				return cardGridPageDataItem{
					Href:     fmt.Sprintf(discordGuildsPath+"/%d", guild.ID),
					Name:     name,
					ImageURL: cmp.Or(guild.IconURL, discordLogoURL),
					ImageAlt: discordGuildImageAlt(name),
				}
			}),
		}, nil
	}))
}

func registerDiscordGuildPage(s *Server, mux *http.ServeMux) {
	mux.Handle("GET "+discordGuildsPath+"/{guildID}", newPageHandlerWithParams(s, discordGuildTemplate, func(r *http.Request) (uint64, error) {
		return strconv.ParseUint(r.PathValue("guildID"), 10, 64)
	}, func(tx *db.Tx, guildID uint64) (any, error) {
		type discordGuildPageData struct {
			Header timelinePageHeader
			*storediscord.GetGuildPageDataResult
		}

		res, err := storediscord.GetGuildPageData(tx, guildID)
		if err != nil {
			return nil, err
		}

		name := displayNameStrOrID(res.Guild.Name, res.Guild.ID)

		return &discordGuildPageData{
			Header: timelinePageHeader{
				Name:     name,
				ImageURL: res.Guild.IconURL,
				ImageAlt: discordGuildImageAlt(name),
			},
			GetGuildPageDataResult: res,
		}, nil
	}))
}
