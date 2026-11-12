# cron-stb

Lightweight cron daemon and job runner for STB / ARM Linux (Armbian, CasaOS).

## Jobs

| Name | Schedule (WIB) | Target |
| :--- | :--- | :--- |
| `sync_progress` | `0 6 * * *` | Discord `#airdrop-progress` |
| `sync_active` | `0 8 * * *` | Discord `#airdrop-active` |
| `archive_monthly` | `0 18 * * *` | Moves rows to Trash (monthly) |
| `alert_airdrop_h1`| `0 21 * * *` | Discord `#airdrop-alerts` |

## Setup

1. Copy `.env.example` to `.env` and fill in your credentials:
   ```bash
   cp .env.example .env
   ```
2. Put `token.json` and `credentials.json` into `./data/`.

## Running with Docker (CasaOS)

```bash
docker compose up -d --build
```

Logs:
```bash
docker compose logs -f
```

## Running Directly

Build:
```bash
go build -ldflags="-s -w" -o cron-stb ./cmd/cron-stb
```

CLI usage:
```bash
# Start daemon
./cron-stb daemon

# List jobs
./cron-stb list

# Run a single job
./cron-stb run sync_active

# Dry-run without posting to Discord
./cron-stb -dry-run run sync_active
```
