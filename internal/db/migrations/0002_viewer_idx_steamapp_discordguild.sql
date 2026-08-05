CREATE INDEX "discord_session_voice_by_channel_idx" ON "discord_session_voice" (
    "channel_id"
);

CREATE INDEX "steam_session_app_by_app_idx" ON "steam_session_app" (
    "app_id"
);
