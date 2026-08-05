-- What changes:
--   * steam_user / discord_user / discord_guild / discord_channel ids are replaced with 1, 2, 3...
--   * steam/discord usernames, discord guild names and discord channel names come from pre-defined pools
--   * avatar/icon urls become random photo urls
--   * steam profile urls all become Gaben's
--   * discord activity name/details/state text is replaced with random ones
--
-- What is preserved:
--   * every timestamp
--   * every row, in every table
--   * steam apps (public data)

PRAGMA foreign_keys = OFF;

BEGIN IMMEDIATE;

-- ===== FAKE DATA (hallucinated by Claude) =====

CREATE TEMP TABLE anon_username (idx INTEGER PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO anon_username (value) VALUES
    ('Lorrem Ipsom'),('PixelPirate'),('NoScopeNancy'),('LagLord'),
    ('CriticalMiss'),('RespawnRoy'),('GhostPepper'),('VelvetHammer'),
    ('CopperKettle'),('NeonNomad'),('QuietRiot'),('MidnightMoth'),
    ('RustyCompass'),('PaperTiger'),('SolarFlare'),('TinCanTom'),
    ('GlassCannon'),('IronBadger'),('SilentSiren'),('AmberWolf'),
    ('CrimsonQuill'),('DustyBoots'),('EchoChamber'),('FrostByte'),
    ('GoldenGoose'),('HollowPoint'),('IvoryTower'),('JadeFalcon'),
    ('KiteString'),('LanternFish'),('MarbleMouse'),('NorthernPike'),
    ('OrbitDecay'),('PlasmaPuffin'),('QuartzQuokka'),('RadioSilence'),
    ('SaltFlats'),('TidalLock'),('UmberOwl'),('VioletStatic'),
    ('WalnutWizard'),('ZephyrZebra'),('BrokenRadio'),('CactusJack'),
    ('DoubleDipper'),('ElasticBand'),('FlatWhite'),('GravelRoad'),
    ('HalfMeasure'),('InkBlot'),('JellyBean'),('KettleCorn'),
    ('LimeLight'),('MossyRock'),('NightOwl'),('OpenSauce'),
    ('PocketLint'),('QuarterPast'),('RubberDuck'),('SlowClap'),
    ('ThumbTack'),('UnderScore'),('WetSocks'),('YardSale');

CREATE TEMP TABLE anon_guild_name (idx INTEGER PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO anon_guild_name (value) VALUES
    ('The Back Room'),('Couch Coop'),('Basement Dwellers'),('Weekend Warriors'),
    ('The Lobby'),('Snack Table'),('Quiet Corner'),('Night Shift'),
    ('Lan Party 1999'),('The Group Chat'),('Fireside'),('Loose Ends'),
    ('Second Breakfast'),('The Hangar'),('Idle Hands'),('Coffee Break'),
    ('Terminal Velocity'),('The Sandbox'),('Low Effort'),('After Hours'),
    ('Rainy Day Club'),('The Workshop'),('Slow Lane'),('Open Invite');

CREATE TEMP TABLE anon_channel_name (idx INTEGER PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO anon_channel_name (value) VALUES
    ('general'),('off-topic'),('gaming'),('music'),
    ('memes'),('links'),('help'),('announcements'),
    ('screenshots'),('cooking'),('secret1'),('valheim'),
    ('General'),('Gaming 1'),('Gaming 2'),('Chill'),
    ('AFK'),('Study Room'),('Movie Night'),('Karaoke'),
    ('Lounge'),('Overflow'),('Quiet'),('Stage');

CREATE TEMP TABLE anon_app_name (idx INTEGER PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO anon_app_name (value) VALUES
    ('Counter-Strike 2'),('Rocket League'),('Visual Studio Code'),('Spotify'),
    ('League of Legends'),('Minecraft'),('Terraria'),('Stardew Valley'),
    ('Dota 2'),('Valorant'),('Apex Legends'),('Overwatch 2'),
    ('Deep Rock Galactic'),('Factorio'),('RimWorld'),('Hades'),
    ('Elden Ring'),('Baldur''s Gate 3'),('Cyberpunk 2077'),('The Witcher 3'),
    ('Portal 2'),('Half-Life: Alyx'),('Team Fortress 2'),('Garry''s Mod'),
    ('Among Us'),('Fall Guys'),('Phasmophobia'),('Lethal Company'),
    ('Euro Truck Simulator 2'),('Cities: Skylines'),('Kerbal Space Program'),('Satisfactory'),
    ('Europa Universalis IV'),('Counter-Strike Source'),('Crusader Kings II'),('Battlefield 6'),
    ('Google Chrome'),('Battlefield 4'),('Spacewar'),('Hearts of Iron IV'),
    ('Warcraft III'),('World of Warcraft'),('Valheim'),('OBS Studio'),
    ('Blender'),('FC 26'),('Hearthstone'),('Custom Status');

-- details/state require a lot of values - they are put together by stitching multiple strings
CREATE TEMP TABLE anon_detail_a (idx INTEGER PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO anon_detail_a (value) VALUES
    ('Neon'),('Velvet'),('Paper'),('Midnight'),('Copper'),('Silent'),('Golden'),('Hollow'),
    ('Amber'),('Crimson'),('Dusty'),('Echo'),('Frost'),('Glass'),('Iron'),('Jade'),
    ('Lantern'),('Marble'),('Northern'),('Orbit'),('Plasma'),('Quartz'),('Radio'),('Salt'),
    ('Tidal'),('Umber'),('Violet'),('Walnut'),('Yellow'),('Zephyr'),('Broken'),('Crystal'),
    ('Distant'),('Electric'),('Faded'),('Gentle'),('Hidden'),('Idle'),('Lucid'),('Molten');

CREATE TEMP TABLE anon_detail_b (idx INTEGER PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO anon_detail_b (value) VALUES
    ('Skyline'),('Machine'),('Ghosts'),('Avenue'),('Harbor'),('Signal'),('Garden'),('Circuit'),
    ('Anthem'),('Mirage'),('Cascade'),('Beacon'),('Voyage'),('Ember'),('Meadow'),('Prism'),
    ('Current'),('Hollow'),('Cabin'),('Drift'),('Falls'),('Grove'),('Haven'),('Island'),
    ('Junction'),('Kite'),('Ledge'),('Mountain'),('Nebula'),('Orchard'),('Passage'),('Quarry'),
    ('Ridge'),('Summit'),('Tunnel'),('Valley'),('Window'),('Yard'),('Zenith'),('Bridge');

CREATE TEMP TABLE anon_state_a (idx INTEGER PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO anon_state_a (value) VALUES
    ('Wandering'),('Resting'),('Queued'),('Ranked'),('Casual'),('Solo'),('Duo'),('Squad'),
    ('Idle'),('Focused'),('Roaming'),('Standby'),('Practicing'),('Exploring'),('Building'),('Browsing'),
    ('Listening'),('Watching'),('Chilling'),('Grinding'),('Farming'),('Scouting'),('Defending'),('Attacking'),
    ('Repairing'),('Crafting'),('Trading'),('Flying'),('Diving'),('Climbing'),('Racing'),('Drifting'),
    ('Sniping'),('Healing'),('Tanking'),('Support'),('Midlane'),('Toplane'),('Jungle'),('Backline');

CREATE TEMP TABLE anon_state_b (idx INTEGER PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO anon_state_b (value) VALUES
    ('Lobby'),('Arena'),('Circuit'),('District'),('Outpost'),('Harbor'),('Station'),('Foundry'),
    ('Terrace'),('Basin'),('Ridge'),('Vault'),('Atrium'),('Bazaar'),('Canyon'),('Dunes'),
    ('Estate'),('Forge'),('Gallery'),('Hangar'),('Inlet'),('Junction'),('Keep'),('Landing'),
    ('Mill'),('Nook'),('Observatory'),('Plaza'),('Quarters'),('Refinery'),('Sector'),('Terminal'),
    ('Undercroft'),('Vista'),('Warehouse'),('Yard'),('Zone'),('Bastion'),('Causeway'),('Depot');

-- ===== ID MAPS =====

CREATE TEMP TABLE steam_user_shuffle AS SELECT id AS old_id, random() AS sort_key FROM steam_user;
CREATE TEMP TABLE steam_user_map (old_id INTEGER PRIMARY KEY, new_id INTEGER NOT NULL UNIQUE);
INSERT INTO steam_user_map (old_id, new_id)
SELECT old_id, ROW_NUMBER() OVER (ORDER BY sort_key, old_id) FROM steam_user_shuffle;

CREATE TEMP TABLE discord_guild_shuffle AS SELECT id AS old_id, random() AS sort_key FROM discord_guild;
CREATE TEMP TABLE discord_guild_map (old_id INTEGER PRIMARY KEY, new_id INTEGER NOT NULL UNIQUE);
INSERT INTO discord_guild_map (old_id, new_id)
SELECT old_id, ROW_NUMBER() OVER (ORDER BY sort_key, old_id) FROM discord_guild_shuffle;

CREATE TEMP TABLE discord_channel_shuffle AS SELECT id AS old_id, random() AS sort_key FROM discord_channel;
CREATE TEMP TABLE discord_channel_map (old_id INTEGER PRIMARY KEY, new_id INTEGER NOT NULL UNIQUE);
INSERT INTO discord_channel_map (old_id, new_id)
SELECT old_id, ROW_NUMBER() OVER (ORDER BY sort_key, old_id) FROM discord_channel_shuffle;

CREATE TEMP TABLE discord_user_shuffle AS SELECT id AS old_id, random() AS sort_key FROM discord_user;
CREATE TEMP TABLE discord_user_map (old_id INTEGER PRIMARY KEY, new_id INTEGER NOT NULL UNIQUE);
INSERT INTO discord_user_map (old_id, new_id)
SELECT old_id, ROW_NUMBER() OVER (ORDER BY sort_key, old_id) FROM discord_user_shuffle;

-- ===== ENTITIES =====

CREATE TEMP TABLE steam_user_new AS
SELECT
    m.new_id AS id,
    u.enabled AS enabled,
    CASE WHEN u.name = '' THEN '' ELSE
        (SELECT value FROM anon_username WHERE idx = ((m.new_id - 1) % (SELECT count(*) FROM anon_username)) + 1)
        || CASE WHEN m.new_id > (SELECT count(*) FROM anon_username)
                THEN CAST((m.new_id - 1) / (SELECT count(*) FROM anon_username) AS TEXT) ELSE '' END
    END AS name,
    CASE WHEN u.profile_url = '' THEN '' ELSE 'https://steamcommunity.com/id/gabelogannewell' END AS profile_url,
    CASE WHEN u.avatar_url = '' THEN '' ELSE 'https://picsum.photos/seed/su' || m.new_id || '/184/184' END AS avatar_url,
    u.extra_info_updated_at AS extra_info_updated_at
FROM steam_user u
JOIN steam_user_map m ON m.old_id = u.id;

DELETE FROM steam_user;
INSERT INTO steam_user (id, enabled, name, profile_url, avatar_url, extra_info_updated_at)
SELECT id, enabled, name, profile_url, avatar_url, extra_info_updated_at FROM steam_user_new;

CREATE TEMP TABLE discord_guild_new AS
SELECT
    m.new_id AS id,
    CASE WHEN g.name = '' THEN '' ELSE
        (SELECT value FROM anon_guild_name WHERE idx = ((m.new_id - 1) % (SELECT count(*) FROM anon_guild_name)) + 1)
        || CASE WHEN m.new_id > (SELECT count(*) FROM anon_guild_name)
                THEN ' ' || CAST((m.new_id - 1) / (SELECT count(*) FROM anon_guild_name) AS TEXT) ELSE '' END
    END AS name,
    CASE WHEN g.icon_url = '' THEN '' ELSE 'https://picsum.photos/seed/dg' || m.new_id || '/256/256' END AS icon_url,
    g.extra_info_updated_at AS extra_info_updated_at
FROM discord_guild g
JOIN discord_guild_map m ON m.old_id = g.id;

DELETE FROM discord_guild;
INSERT INTO discord_guild (id, name, icon_url, extra_info_updated_at)
SELECT id, name, icon_url, extra_info_updated_at FROM discord_guild_new;

CREATE TEMP TABLE discord_channel_new AS
SELECT
    m.new_id AS id,
    (SELECT new_id FROM discord_guild_map WHERE old_id = c.guild_id) AS guild_id,
    CASE WHEN c.name = '' THEN '' ELSE
        (SELECT value FROM anon_channel_name WHERE idx = ((m.new_id - 1) % (SELECT count(*) FROM anon_channel_name)) + 1)
        || CASE WHEN m.new_id > (SELECT count(*) FROM anon_channel_name)
                THEN ' ' || CAST((m.new_id - 1) / (SELECT count(*) FROM anon_channel_name) AS TEXT) ELSE '' END
    END AS name,
    c.extra_info_updated_at AS extra_info_updated_at
FROM discord_channel c
JOIN discord_channel_map m ON m.old_id = c.id;

DELETE FROM discord_channel;
INSERT INTO discord_channel (id, guild_id, name, extra_info_updated_at)
SELECT id, guild_id, name, extra_info_updated_at FROM discord_channel_new;

CREATE TEMP TABLE discord_user_new AS
SELECT
    m.new_id AS id,
    (SELECT new_id FROM discord_guild_map WHERE old_id = u.presence_guild_id) AS presence_guild_id,
    u.enabled AS enabled,
    CASE WHEN u.name = '' THEN '' ELSE
        (SELECT value FROM anon_username WHERE idx = ((m.new_id - 1) % (SELECT count(*) FROM anon_username)) + 1)
        || CASE WHEN m.new_id > (SELECT count(*) FROM anon_username)
                THEN CAST((m.new_id - 1) / (SELECT count(*) FROM anon_username) AS TEXT) ELSE '' END
    END AS name,
    CASE WHEN u.avatar_url = '' THEN '' ELSE 'https://picsum.photos/seed/du' || m.new_id || '/256/256' END AS avatar_url,
    u.extra_info_updated_at AS extra_info_updated_at
FROM discord_user u
JOIN discord_user_map m ON m.old_id = u.id;

DELETE FROM discord_user;
INSERT INTO discord_user (id, presence_guild_id, enabled, name, avatar_url, extra_info_updated_at)
SELECT id, presence_guild_id, enabled, name, avatar_url, extra_info_updated_at FROM discord_user_new;

-- ===== ACTIVITY =====

CREATE TEMP TABLE discord_activity_name_new AS
SELECT old_id, CASE WHEN was_empty THEN '' ELSE
    (SELECT value FROM anon_app_name WHERE idx = ((slot - 1) % (SELECT count(*) FROM anon_app_name)) + 1)
    || CASE WHEN slot > (SELECT count(*) FROM anon_app_name)
            THEN ' ' || CAST((slot - 1) / (SELECT count(*) FROM anon_app_name) AS TEXT) ELSE '' END
END AS value
FROM (
    SELECT old_id, was_empty, ROW_NUMBER() OVER (ORDER BY sort_key, old_id) AS slot
    FROM (SELECT id AS old_id, name = '' AS was_empty, random() AS sort_key FROM discord_activity_name)
);

UPDATE discord_activity_name SET name = '__anon_tmp_' || id;
UPDATE discord_activity_name
SET name = (SELECT value FROM discord_activity_name_new WHERE old_id = discord_activity_name.id);

CREATE TEMP TABLE discord_activity_details_new AS
SELECT old_id,
    (SELECT value FROM anon_detail_a WHERE idx = ((slot - 1) % (SELECT count(*) FROM anon_detail_a)) + 1)
    || ' ' ||
    (SELECT value FROM anon_detail_b WHERE idx = ((((slot - 1) / (SELECT count(*) FROM anon_detail_a)) + ((slot - 1) % (SELECT count(*) FROM anon_detail_a)) * 7) % (SELECT count(*) FROM anon_detail_b)) + 1)
    || CASE WHEN slot > (SELECT count(*) FROM anon_detail_a) * (SELECT count(*) FROM anon_detail_b)
            THEN ' ' || CAST((slot - 1) / ((SELECT count(*) FROM anon_detail_a) * (SELECT count(*) FROM anon_detail_b)) AS TEXT) ELSE '' END
    AS value
FROM (
    SELECT old_id, ROW_NUMBER() OVER (ORDER BY sort_key, old_id) AS slot
    FROM (SELECT id AS old_id, random() AS sort_key FROM discord_activity_details)
);

UPDATE discord_activity_details SET details = '__anon_tmp_' || id;
UPDATE discord_activity_details
SET details = (SELECT value FROM discord_activity_details_new WHERE old_id = discord_activity_details.id);

CREATE TEMP TABLE discord_activity_state_new AS
SELECT old_id,
    (SELECT value FROM anon_state_a WHERE idx = ((slot - 1) % (SELECT count(*) FROM anon_state_a)) + 1)
    || ' ' ||
    (SELECT value FROM anon_state_b WHERE idx = ((((slot - 1) / (SELECT count(*) FROM anon_state_a)) + ((slot - 1) % (SELECT count(*) FROM anon_state_a)) * 7) % (SELECT count(*) FROM anon_state_b)) + 1)
    || CASE WHEN slot > (SELECT count(*) FROM anon_state_a) * (SELECT count(*) FROM anon_state_b)
            THEN ' ' || CAST((slot - 1) / ((SELECT count(*) FROM anon_state_a) * (SELECT count(*) FROM anon_state_b)) AS TEXT) ELSE '' END
    AS value
FROM (
    SELECT old_id, ROW_NUMBER() OVER (ORDER BY sort_key, old_id) AS slot
    FROM (SELECT id AS old_id, random() AS sort_key FROM discord_activity_state)
);

UPDATE discord_activity_state SET state = '__anon_tmp_' || id;
UPDATE discord_activity_state
SET state = (SELECT value FROM discord_activity_state_new WHERE old_id = discord_activity_state.id);

-- ============================================================================
-- ===== SESSIONS =====
-- ============================================================================

UPDATE steam_session_persona_state
SET user_id = (SELECT new_id FROM steam_user_map WHERE old_id = user_id);

UPDATE steam_session_app
SET user_id = (SELECT new_id FROM steam_user_map WHERE old_id = user_id);

UPDATE discord_session_status
SET user_id  = (SELECT new_id FROM discord_user_map WHERE old_id = user_id),
    guild_id = (SELECT new_id FROM discord_guild_map WHERE old_id = guild_id);

UPDATE discord_session_activity
SET user_id  = (SELECT new_id FROM discord_user_map WHERE old_id = user_id),
    guild_id = (SELECT new_id FROM discord_guild_map WHERE old_id = guild_id);

UPDATE discord_session_voice
SET user_id    = (SELECT new_id FROM discord_user_map WHERE old_id = user_id),
    channel_id = (SELECT new_id FROM discord_channel_map WHERE old_id = channel_id);

COMMIT;

-- ===== CLEANUP + INTEGRITY CHECKS

DROP TABLE steam_user_shuffle;
DROP TABLE steam_user_map;
DROP TABLE steam_user_new;
DROP TABLE discord_guild_shuffle;
DROP TABLE discord_guild_map;
DROP TABLE discord_guild_new;
DROP TABLE discord_channel_shuffle;
DROP TABLE discord_channel_map;
DROP TABLE discord_channel_new;
DROP TABLE discord_user_shuffle;
DROP TABLE discord_user_map;
DROP TABLE discord_user_new;
DROP TABLE discord_activity_name_new;
DROP TABLE discord_activity_details_new;
DROP TABLE discord_activity_state_new;
DROP TABLE anon_username;
DROP TABLE anon_guild_name;
DROP TABLE anon_channel_name;
DROP TABLE anon_app_name;
DROP TABLE anon_detail_a;
DROP TABLE anon_detail_b;
DROP TABLE anon_state_a;
DROP TABLE anon_state_b;

PRAGMA foreign_key_check;
PRAGMA integrity_check;

VACUUM;
