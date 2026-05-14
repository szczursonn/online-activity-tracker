CREATE TABLE "heartbeat" (
    "key"			TEXT NOT NULL,
    "timestamp"		INTEGER NOT NULL,
    PRIMARY KEY("key")
) WITHOUT ROWID,STRICT;

CREATE TABLE "steam_user" (
    "id"						INTEGER NOT NULL,
    "enabled"					INTEGER NOT NULL,
    "name"						TEXT NOT NULL DEFAULT '',
    "profile_url"				TEXT NOT NULL DEFAULT '',
    "avatar_url"				TEXT NOT NULL DEFAULT '',
    "extra_info_updated_at"		INTEGER,
    PRIMARY KEY("id")
) WITHOUT ROWID,STRICT;

CREATE INDEX "steam_user_enablement_idx" ON "steam_user" (
    "enabled"
);

CREATE TABLE "steam_session_persona_state" (
    "id"					INTEGER PRIMARY KEY,
    "user_id"				INTEGER NOT NULL,
    "persona_state"			INTEGER NOT NULL,
    "first_observed_at"		INTEGER NOT NULL,
    "last_observed_at"		INTEGER,
    FOREIGN KEY("user_id")	REFERENCES "steam_user"("id")
) STRICT;

CREATE INDEX "steam_session_persona_state_open_idx" ON "steam_session_persona_state" (
    "user_id"
) WHERE "last_observed_at" IS NULL;

CREATE INDEX "steam_session_persona_state_by_user_idx" ON "steam_session_persona_state" (
    "user_id"
);

CREATE TABLE "steam_app" (
    "id"							INTEGER NOT NULL,
    "name"							TEXT NOT NULL DEFAULT '',
    "header_image_url"				TEXT NOT NULL DEFAULT '',
    "extra_info_updated_at"			INTEGER,
    PRIMARY KEY("id")
) WITHOUT ROWID,STRICT;

CREATE INDEX "steam_app_extra_info_updated_at_idx" ON "steam_app" (
    "extra_info_updated_at"
);

CREATE TABLE "steam_session_app" (
    "id"					INTEGER PRIMARY KEY,
    "user_id"				INTEGER NOT NULL,
    "app_id"				INTEGER NOT NULL,
    "first_observed_at"		INTEGER NOT NULL,
    "last_observed_at"		INTEGER,
    FOREIGN KEY("user_id")	REFERENCES "steam_user"("id"),
    FOREIGN KEY("app_id")	REFERENCES "steam_app"("id")
) STRICT;

CREATE INDEX "steam_session_app_open_idx" ON "steam_session_app" (
    "user_id"
) WHERE "last_observed_at" IS NULL;

CREATE INDEX "steam_session_app_by_user_idx" ON "steam_session_app" (
    "user_id"
);

CREATE VIEW steam_session_persona_state_debug_vw AS
SELECT
COALESCE(NULLIF(su.name, ''), su.id) AS user,
datetime(ssps.first_observed_at / 1000, 'unixepoch') AS first_observed_at_utc,
datetime(ssps.last_observed_at / 1000, 'unixepoch') AS last_observed_at_utc,
ssps.persona_state AS persona_state
FROM steam_session_persona_state ssps
LEFT JOIN steam_user su ON ssps.user_id = su.id
ORDER BY ssps.first_observed_at DESC, ssps.user_id ASC;

CREATE VIEW steam_session_app_debug_vw AS
SELECT
COALESCE(NULLIF(su.name,''), su.id) AS user,
datetime(ssa.first_observed_at / 1000, 'unixepoch') AS first_observed_at_utc,
datetime(ssa.last_observed_at / 1000, 'unixepoch') AS last_observed_at_utc,
COALESCE(NULLIF(sa.name,''), sa.id) AS app
FROM steam_session_app ssa
LEFT JOIN steam_user su ON ssa.user_id = su.id
LEFT JOIN steam_app sa ON ssa.app_id = sa.id
ORDER BY ssa.first_observed_at DESC, ssa.user_id ASC;

CREATE TABLE "discord_guild" (
    "id"							INTEGER NOT NULL,
    "name"							TEXT NOT NULL DEFAULT '',
    "icon_url"						TEXT NOT NULL DEFAULT '',
    "extra_info_updated_at"			INTEGER,
    PRIMARY KEY("id")
) WITHOUT ROWID,STRICT;

CREATE TABLE "discord_channel" (
    "id"							INTEGER NOT NULL,
    "guild_id"						INTEGER NOT NULL,
    "name"							TEXT NOT NULL DEFAULT '',
    "extra_info_updated_at"			INTEGER,
    PRIMARY KEY("id"),
    FOREIGN KEY("guild_id") 		REFERENCES "discord_guild"("id")
) WITHOUT ROWID,STRICT;

CREATE INDEX "discord_channel_by_guild_idx" ON "discord_channel" (
    "guild_id"
);

CREATE TABLE "discord_user" (
    "id"								INTEGER NOT NULL,
    "presence_guild_id"					INTEGER NOT NULL,
    "enabled"							INTEGER NOT NULL,
    "name"								TEXT NOT NULL DEFAULT '',
    "avatar_url"						TEXT NOT NULL DEFAULT '',
    "extra_info_updated_at"				INTEGER,
    PRIMARY KEY("id"),
    FOREIGN KEY("presence_guild_id") 	REFERENCES "discord_guild"("id")
) WITHOUT ROWID,STRICT;

CREATE INDEX "discord_user_enablement_idx" ON "discord_user" (
    "enabled"
);

CREATE TABLE "discord_session_status" (
    "id"						INTEGER PRIMARY KEY,
    "user_id"					INTEGER NOT NULL,
    "guild_id"					INTEGER NOT NULL,
    "status_desktop"			INTEGER NOT NULL,
    "status_mobile"				INTEGER NOT NULL,
    "status_web"				INTEGER NOT NULL,
    "start_observed_at"			INTEGER NOT NULL,
    "end_observed_at"			INTEGER,
    FOREIGN KEY("user_id") 		REFERENCES "discord_user"("id"),
    FOREIGN KEY("guild_id") 	REFERENCES "discord_guild"("id")
) STRICT;

CREATE INDEX "discord_session_status_open_by_guild_idx" ON "discord_session_status" (
    "guild_id"
) WHERE "end_observed_at" IS NULL;

CREATE INDEX "discord_session_status_open_by_user_idx" ON "discord_session_status" (
    "user_id"
) WHERE "end_observed_at" IS NULL;

CREATE INDEX "discord_session_status_by_user_idx" ON "discord_session_status" (
    "user_id"
);

CREATE TABLE "discord_activity_name" (
    "id"	INTEGER PRIMARY KEY,
    "name"	TEXT NOT NULL UNIQUE
) STRICT;

CREATE UNIQUE INDEX "discord_activity_name_idx" ON "discord_activity_name" (
    "name"
);

CREATE TABLE "discord_activity_details" (
    "id"		INTEGER PRIMARY KEY,
    "details"	TEXT NOT NULL UNIQUE
) STRICT;

CREATE UNIQUE INDEX "discord_activity_details_idx" ON "discord_activity_details" (
    "details"
);

CREATE TABLE "discord_activity_state" (
    "id"	INTEGER PRIMARY KEY,
    "state"	TEXT NOT NULL UNIQUE
) STRICT;

CREATE UNIQUE INDEX "discord_activity_state_idx" ON "discord_activity_state" (
    "state"
);

CREATE TABLE "discord_session_activity" (
    "id"						INTEGER PRIMARY KEY,
    "user_id"					INTEGER NOT NULL,
    "guild_id"					INTEGER NOT NULL,
    "name_id"					INTEGER NOT NULL,
    "details_id"				INTEGER,
    "state_id"					INTEGER,
    "start_observed_at"			INTEGER NOT NULL,
    "end_observed_at"			INTEGER,
    FOREIGN KEY("user_id") 		REFERENCES "discord_user"("id"),
    FOREIGN KEY("guild_id")		REFERENCES "discord_guild"("id"),
    FOREIGN KEY("name_id")		REFERENCES "discord_activity_name"("id"),
    FOREIGN KEY("details_id") 	REFERENCES "discord_activity_details"("id"),
    FOREIGN KEY("state_id") 	REFERENCES "discord_activity_state"("id")
) STRICT;

CREATE INDEX "discord_session_activity_open_by_guild_idx" ON "discord_session_activity" (
    "guild_id"
) WHERE "end_observed_at" IS NULL;

CREATE INDEX "discord_session_activity_open_by_user_idx" ON "discord_session_activity" (
    "user_id"
) WHERE "end_observed_at" IS NULL;

CREATE INDEX "discord_session_activity_by_user_idx" ON "discord_session_activity" (
    "user_id"
);

CREATE TABLE "discord_session_voice" (
    "id"						INTEGER PRIMARY KEY,
    "user_id"					INTEGER NOT NULL,
    "channel_id"				INTEGER NOT NULL,
    "start_observed_at"			INTEGER NOT NULL,
    "end_observed_at"			INTEGER,
    FOREIGN KEY("user_id")		REFERENCES "discord_user"("id"),
    FOREIGN KEY("channel_id")	REFERENCES "discord_channel"("id")
) STRICT;

CREATE INDEX "discord_session_voice_open_by_user_idx" ON "discord_session_voice" (
    "user_id",
    "channel_id"
) WHERE "end_observed_at" IS NULL;

CREATE INDEX "discord_session_voice_open_by_channel_idx" ON "discord_session_voice" (
    "channel_id",
    "user_id"
) WHERE "end_observed_at" IS NULL;

CREATE INDEX "discord_session_voice_by_user_idx" ON "discord_session_voice" (
    "user_id"
);

CREATE VIEW discord_session_status_debug_vw AS
SELECT
COALESCE(NULLIF(du.name,''), du.id) AS user,
COALESCE(NULLIF(dg.name,''), dg.id) AS guild,
datetime(dss.start_observed_at / 1000, 'unixepoch') AS start_observed_at_utc,
datetime(dss.end_observed_at / 1000, 'unixepoch') AS end_observed_at_utc,
dss.status_desktop AS status_desktop,
dss.status_mobile AS status_mobile,
dss.status_web AS status_web
FROM discord_session_status dss
LEFT JOIN discord_user du ON dss.user_id = du.id
LEFT JOIN discord_guild dg ON dss.guild_id = dg.id
ORDER BY dss.start_observed_at DESC, dss.user_id ASC;

CREATE VIEW discord_session_activity_debug_vw AS
SELECT
COALESCE(NULLIF(du.name,''), du.id) AS user,
COALESCE(NULLIF(dg.name,''), dg.id) AS guild,
datetime(dsa.start_observed_at / 1000, 'unixepoch') AS start_observed_at_utc,
datetime(dsa.end_observed_at / 1000, 'unixepoch') AS end_observed_at_utc,
dan.name AS name,
dad.details AS details,
das.state AS state
FROM discord_session_activity dsa
LEFT JOIN discord_user du ON dsa.user_id = du.id
LEFT JOIN discord_guild dg ON dsa.guild_id = dg.id
LEFT JOIN discord_activity_name dan ON dsa.name_id = dan.id
LEFT JOIN discord_activity_details dad ON dsa.details_id = dad.id
LEFT JOIN discord_activity_state das ON dsa.state_id = das.id
ORDER BY dsa.start_observed_at DESC, dsa.user_id ASC;

CREATE VIEW discord_session_voice_debug_vw AS
SELECT
COALESCE(NULLIF(du.name,''), du.id) AS user,
COALESCE(NULLIF(dg.name,''), dg.id) AS guild,
COALESCE(NULLIF(dc.name,''), dc.id) AS channel,
datetime(dsv.start_observed_at / 1000, 'unixepoch') AS start_observed_at_utc,
datetime(dsv.end_observed_at / 1000, 'unixepoch') AS end_observed_at_utc
FROM discord_session_voice dsv
LEFT JOIN discord_user du ON dsv.user_id = du.id
LEFT JOIN discord_channel dc ON dsv.channel_id = dc.id
LEFT JOIN discord_guild dg ON dc.guild_id = dg.id
ORDER BY dsv.start_observed_at DESC, dsv.user_id ASC, dsv.channel_id ASC;
