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
	steamAppsPath  = "/steam-apps"
	steamAppsTitle = "Steam Apps"
)

var steamAppsNavLink = navLink{
	Href:    steamAppsPath,
	Label:   steamAppsTitle,
	IconURL: steamLogoURL,
	IconAlt: "steam icon",
}

var steamAppTemplate = mustParseTemplate(timelinePageTemplateFileName, "steam_app.html")

func steamAppImageAlt(name string) string {
	return fmt.Sprintf("steam app %q logo", name)
}

func registerSteamAppsPage(s *Server, mux *http.ServeMux) {
	mux.Handle("GET "+steamAppsPath, newPageHandler(s, cardGridTemplate, func(tx *db.Tx) (any, error) {
		apps, err := storesteam.GetAllApps(tx)
		if err != nil {
			return nil, err
		}

		return &cardGridPageData{
			Title: steamAppsTitle,
			Items: mapSeq(apps, func(app storesteam.GetAllAppsApp) cardGridPageDataItem {
				name := displayNameStrOrID(app.Name, app.ID)

				return cardGridPageDataItem{
					Href:     fmt.Sprintf(steamAppsPath+"/%d", app.ID),
					Name:     name,
					ImageURL: cmp.Or(app.HeaderImageURL, steamLogoURL),
					ImageAlt: steamAppImageAlt(name),
				}
			}),
		}, nil
	}))
}

func registerSteamAppPage(s *Server, mux *http.ServeMux) {
	mux.Handle("GET "+steamAppsPath+"/{appID}", newPageHandlerWithParams(s, steamAppTemplate, func(r *http.Request) (uint64, error) {
		return strconv.ParseUint(r.PathValue("appID"), 10, 64)
	}, func(tx *db.Tx, appID uint64) (any, error) {
		type steamAppPageData struct {
			Header timelinePageHeader
			*storesteam.GetAppPageDataResult
		}

		res, err := storesteam.GetAppPageData(tx, appID)
		if err != nil {
			return nil, err
		}

		name := displayNameStrOrID(res.App.Name, res.App.ID)

		return &steamAppPageData{
			Header: timelinePageHeader{
				Name:     name,
				ImageURL: res.App.HeaderImageURL,
				ImageAlt: steamAppImageAlt(name),
				URL:      fmt.Sprintf("https://store.steampowered.com/app/%d/", res.App.ID),
				URLLabel: "Steam store page",
			},
			GetAppPageDataResult: res,
		}, nil
	}))
}
