package config

import (
	"strings"
	"time"
)

const defaultLogsFormat = LogFormatJSON
const defaultLogsManageFormat = LogFormatSimpleText
const defaultDatabasePath = "./oat.db"
const defaultSteamPollInterval = 2 * time.Minute
const defaultSteamAppExtraInfoStaleCheckInterval = time.Minute
const defaultSteamAppExtraInfoStaleTime = 72 * time.Hour
const defaultViewerListenAddr = ":8080"

type Config struct {
	Logs     Logs
	Database Database
	Steam    Steam
	Discord  Discord
	Viewer   Viewer
}

type LogFormat string

const (
	LogFormatJSON       LogFormat = "json"
	LogFormatText       LogFormat = "text"
	LogFormatSimpleText LogFormat = "simpletext"
)

type Logs struct {
	Debug        bool
	Format       LogFormat
	ManageFormat LogFormat
}

type Database struct {
	Path string
}

type Steam struct {
	Key                            string
	PollInterval                   time.Duration
	AppExtraInfoStaleCheckInterval time.Duration
	AppExtraInfoStaleTime          time.Duration
}

type Discord struct {
	Token string
}

type Viewer struct {
	ListenAddr string
}

func parseLogFormat(value string, defaultValue LogFormat) LogFormat {
	typedValue := LogFormat(strings.TrimSpace(value))

	switch typedValue {
	case LogFormatJSON, LogFormatText, LogFormatSimpleText:
		return typedValue
	default:
		return defaultValue
	}
}

func trimAndApplyDefaultString(value string, defaultValue string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue
	}
	return value
}

func applyDefaultPositiveDuration(value time.Duration, defaultValue time.Duration) time.Duration {
	if value <= 0 {
		return defaultValue
	}
	return value
}
