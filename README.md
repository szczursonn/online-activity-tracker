# online-activity-tracker

Steam and Discord activity tracker

![discord timeline](/docs/discord_timeline.png)

The app collects the following data:

1. Steam User's Persona State (Online, AFK, Busy etc.)
2. Steam User's Active App (which app/game the user is in)
3. Steam User's extra info (name, avatar)
4. Steam App's extra info (name, image)
5. Discord User's Status (Online, AFK, Busy etc.)
6. Discord User's Activities (In Game X, Listening to Y)
7. Discord User's Voice Channel activity (joining/leaving Voice Channels)
8. Discord User's extra info (name, avatar)
9. Discord Guild's extra info (name, icon)
10. Discord Channel's extra info (name)

Data is saved to a [SQLite](https://sqlite.org/) database in WAL mode ([can easily backup](https://litestream.io/))

## Running

1. Configure secrets in `oat.toml`:

```toml
# Required for Steam tracking to work
[steam]
key = "abc"

# Required for Discord tracking to work
[discord]
token = "a.b.c"
```

2. Run DB migrations

```bash
oat migrate
```

3. Configure tracked users

```bash
# You can run these while tracking is in progress!

# Add Steam user by SteamID64
oat steam enable 123

# Add Discord user by their ID + Guild ID from which presence data should be collected
oat discord enable 123 321

# Disable users - this will only stop tracking, it will not delete existing data
oat steam disable 123
oat discord disable 123
```

4. Run services

```bash
# steam - steam tracking
# discord - discord tracking
# view - basic HTTP server for exploring data

# Run as seperate processes
oat run steam
oat run discord

# Or as a single process
oat run steam discord view
```

## Build

```bash
# CGO free! 🎉🎉🎉
go build -o oat cmd/app/main.go
```

## Docker

Config and database live inside `/data` mount.

1. Create a `./data` directory and put `oat.toml` inside it. Ensure uid `65532`, has read/write access to it:

```bash
mkdir -p data
sudo chown -R 65532:65532 data
```

2. Build the image:

```bash
docker build -t oat .
```

3. Follow setup steps 2-3 in [Running](#running), replacing `oat` with `docker run --rm -v ./data:/data oat`

4. Start the container (long-lived):

```bash
docker run -d --name oat \
  -p 8080:8080 \
  -v ./data:/data \
  --restart unless-stopped \
  oat run steam discord view
```

(port mapping is not required if not using `view` service)
