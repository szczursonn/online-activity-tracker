package config

import (
	"time"

	"github.com/BurntSushi/toml"
)

type rawTOMLConfig struct {
	Logs struct {
		Debug        bool   `toml:"debug"`
		Format       string `toml:"format"`
		ManageFormat string `toml:"manage_format"`
	} `toml:"logs"`
	Database struct {
		Path string `toml:"path"`
	} `toml:"database"`
	Steam struct {
		Key                            string        `toml:"key"`
		PollInterval                   time.Duration `toml:"poll_interval"`
		AppExtraInfoStaleCheckInterval time.Duration `toml:"app_extra_info_stale_check_interval"`
		AppExtraInfoStaleTime          time.Duration `toml:"app_extra_info_stale_time"`
	} `toml:"steam"`
	Discord struct {
		Token string `toml:"token"`
	} `toml:"discord"`
	Viewer struct {
		ListenAddr string `toml:"listen_addr"`
	} `toml:"viewer"`
}

func LoadTOMLFile(filePath string) (*Config, error) {
	rawCfg := &rawTOMLConfig{}
	if _, err := toml.DecodeFile(filePath, rawCfg); err != nil {
		return nil, err
	}

	return &Config{
		Logs: Logs{
			Debug:        rawCfg.Logs.Debug,
			Format:       parseLogFormat(rawCfg.Logs.Format, defaultLogsFormat),
			ManageFormat: parseLogFormat(rawCfg.Logs.ManageFormat, defaultLogsManageFormat),
		},
		Database: Database{
			Path: trimAndApplyDefaultString(rawCfg.Database.Path, defaultDatabasePath),
		},
		Steam: Steam{
			Key:                            trimAndApplyDefaultString(rawCfg.Steam.Key, ""),
			PollInterval:                   applyDefaultPositiveDuration(rawCfg.Steam.PollInterval, defaultSteamPollInterval),
			AppExtraInfoStaleCheckInterval: applyDefaultPositiveDuration(rawCfg.Steam.AppExtraInfoStaleCheckInterval, defaultSteamAppExtraInfoStaleCheckInterval),
			AppExtraInfoStaleTime:          applyDefaultPositiveDuration(rawCfg.Steam.AppExtraInfoStaleTime, defaultSteamAppExtraInfoStaleTime),
		},
		Discord: Discord{
			Token: trimAndApplyDefaultString(rawCfg.Discord.Token, ""),
		},
		Viewer: Viewer{
			ListenAddr: trimAndApplyDefaultString(rawCfg.Viewer.ListenAddr, defaultViewerListenAddr),
		},
	}, nil
}
