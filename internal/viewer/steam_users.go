package viewer

import (
	"cmp"
	"fmt"
	"net/http"
	"strconv"

	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storesteam"
)

const (
	steamUsersPath  = "/steam-users"
	steamUsersTitle = "Steam Users"
)

var steamUsersNavLink = navLink{
	Href:    steamUsersPath,
	Label:   steamUsersTitle,
	IconURL: steamLogoURL,
	IconAlt: "steam icon",
}

var steamUserTemplate = mustParseTemplate(timelinePageTemplateFileName, "steam_user.html")

func steamUserImageAlt(name string) string {
	return fmt.Sprintf("steam user %q avatar", name)
}

func registerSteamUsersPage(s *Server, mux *http.ServeMux) {
	mux.Handle("GET "+steamUsersPath, newPageHandler(s, cardGridTemplate, func(tx *db.Tx) (any, error) {
		users, err := storesteam.GetAllUsers(tx)
		if err != nil {
			return nil, err
		}

		return &cardGridPageData{
			Title: steamUsersTitle,
			Items: mapSeq(users, func(user storesteam.GetAllUsersUser) cardGridPageDataItem {
				name := displayNameStrOrID(user.Name, user.ID)

				return cardGridPageDataItem{
					Href:     fmt.Sprintf(steamUsersPath+"/%d", user.ID),
					Name:     name,
					ImageURL: cmp.Or(user.AvatarURL, steamLogoURL),
					ImageAlt: steamUserImageAlt(name),
				}
			}),
		}, nil
	}))
}

func registerSteamUserPage(s *Server, mux *http.ServeMux) {
	mux.Handle("GET "+steamUsersPath+"/{userID}", newPageHandlerWithParams(s, steamUserTemplate, func(r *http.Request) (uint64, error) {
		return strconv.ParseUint(r.PathValue("userID"), 10, 64)
	}, func(tx *db.Tx, userID uint64) (any, error) {
		type steamUserPageData struct {
			Header timelinePageHeader
			*storesteam.GetUserPageDataResult
		}

		res, err := storesteam.GetUserPageData(tx, userID)
		if err != nil {
			return nil, err
		}

		name := displayNameStrOrID(res.User.Name, res.User.ID)

		return &steamUserPageData{
			Header: timelinePageHeader{
				Name:     name,
				ImageURL: res.User.AvatarURL,
				ImageAlt: steamUserImageAlt(name),
				URL:      res.User.ProfileURL,
				URLLabel: "Steam profile",
			},
			GetUserPageDataResult: res,
		}, nil
	}))
}
