<p align="center">
  <img src="./assets/kotei.png" alt="Kotei Logo" width="128">
</p>

# Kotei

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Kotei** automates monitoring canon anime episodes in Sonarr using data from [AnimeFillerList.com](https://www.animefillerlist.com/).

I couldn't get [SoFE](https://github.com/chkpwd/sofe) to work (skill issue) and was tired of manually looking up canon episodes for series like _Detective Conan_ so I babysitted Gemini to make this.

---

## Features

-   ✅ Fetches canon episode lists from AnimeFillerList.com
-   📡 Updates Sonarr to monitor new episodes based on your config
-   🔍 Optionally triggers searches for monitored episodes
-   🕒 Supports one-time or scheduled runs via cron
-   🐳 Easy Docker deployment

---

## Quick Start (with Docker)

### 1. Create a `config.yaml` file

Place it in the same directory as your `docker-compose.yml`:

```yaml
sonarr:
    baseurl: "YOUR_SONARR_URL_HERE" # REQUIRED: Your Sonarr base URL
    apikey: "YOUR_SONARR_API_KEY_HERE" # REQUIRED: Found in Sonarr > Settings > General

animes:
    - title: "another-anime" # Slug from animefillerlist.com URL
      sonarr_title: "Another Anime Title in Sonarr" # Exact match in Sonarr
      include_canon_types: ["manga", "anime", "mixed"]
      cutoff_episode: 1
      search_enabled: false

schedule:
    cron_spec: "@daily" # Example: Run once daily at midnight
```

---

### 2. Create a `docker-compose.yml` file

```yaml
services:
    kotei:
        image: ghcr.io/bowtie/kotei:latest
        container_name: kotei
        volumes:
            - ./config.yaml:/app/config.yaml:ro
        restart: unless-stopped
        environment:
            - TZ=Etc/UTC
```

### 3. Run Kotei

```sh
docker-compose up -d
```
