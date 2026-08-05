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
	discordUsersPath  = "/discord-users"
	discordUsersTitle = "Discord Users"
)

var discordUsersNavLink = navLink{
	Href:    discordUsersPath,
	Label:   discordUsersTitle,
	IconURL: discordLogoURL,
	IconAlt: "discord icon",
}

var discordUserTemplate = mustParseTemplate(timelinePageTemplateFileName, "discord_user.html")

func discordUserImageAlt(name string) string {
	return fmt.Sprintf("discord user %q avatar", name)
}

func registerDiscordUsersPage(s *Server, mux *http.ServeMux) {
	mux.Handle("GET "+discordUsersPath, newPageHandler(s, cardGridTemplate, func(tx *db.Tx) (any, error) {
		users, err := storediscord.GetAllUsers(tx)
		if err != nil {
			return nil, err
		}

		return &cardGridPageData{
			Title: discordUsersTitle,
			Items: mapSeq(users, func(user storediscord.GetAllUsersUser) cardGridPageDataItem {
				name := displayNameStrOrID(user.Name, user.ID)

				return cardGridPageDataItem{
					Href:     fmt.Sprintf(discordUsersPath+"/%d", user.ID),
					Name:     name,
					ImageURL: cmp.Or(user.AvatarURL, discordLogoURL),
					ImageAlt: discordUserImageAlt(name),
				}
			}),
		}, nil
	}))
}

func registerDiscordUserPage(s *Server, mux *http.ServeMux) {
	mux.Handle("GET "+discordUsersPath+"/{userID}", newPageHandlerWithParams(s, discordUserTemplate, func(r *http.Request) (uint64, error) {
		return strconv.ParseUint(r.PathValue("userID"), 10, 64)
	}, func(tx *db.Tx, userID uint64) (any, error) {
		type discordUserPageData struct {
			Header timelinePageHeader
			*storediscord.GetUserPageDataResult
		}

		res, err := storediscord.GetUserPageData(tx, userID)
		if err != nil {
			return nil, err
		}

		name := displayNameStrOrID(res.User.Name, res.User.ID)

		return &discordUserPageData{
			Header: timelinePageHeader{
				Name:     name,
				ImageURL: res.User.AvatarURL,
				ImageAlt: discordUserImageAlt(name),
				URL:      fmt.Sprintf("https://discord.com/users/%d", res.User.ID),
				URLLabel: "Discord profile",
			},
			GetUserPageDataResult: res,
		}, nil
	}))
}
