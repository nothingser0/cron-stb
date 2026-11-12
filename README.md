# cron-stb

Lightweight cron daemon and job runner for STB / ARM Linux (Armbian, CasaOS).

## Jobs

### Airdrop Automation
| Name | Schedule (WIB) | Target |
| :--- | :--- | :--- |
| `sync_progress` | `0 6 * * *` | Discord `#airdrop-progress` |
| `sync_active` | `0 8 * * *` | Discord `#airdrop-active` |
| `archive_monthly` | `0 18 * * *` | Moves rows to Trash (monthly) |
| `alert_airdrop_h1`| `0 21 * * *` | Discord `#airdrop-alerts` |

### Workspace & Personal Channel Automation
| Name | Schedule (WIB) | Target | Description |
| :--- | :--- | :--- | :--- |
| `schedule_daily` | `0 7 * * *` | Discord `#planning-schedule` | Briefing agenda & deadline hari ini |
| `tasks_daily` | `0 8 * * *` | Discord `#planning-task` | Fokus task & next action hari ini |
| `goals_weekly` | `0 9 * * 1` | Discord `#personal-goals` | Check-in target mingguan (Senin) |
| `learning_weekly`| `0 19 * * 0` | Discord `#learning-milestones` | Rekap belajar mingguan (Minggu) |
| `journal_nightly`| `0 21 * * *` | Discord `#personal-journal` | Pengingat refleksi malam |
| `finance_daily` | `0 22 * * *` | Discord `#finance-inbox` | Rekap harian (hanya kirim jika ada transaksi) |
| `finance_monthly`| `0 8 1 * *` | Discord `#finance-inbox` | Rekap bulanan, arus kas & top pengeluaran |

## Setup

1. Copy `.env.example` to `.env` and fill in your credentials:
   ```bash
   cp .env.example .env
   ```
2. Set `HERMES_WORKSPACE_DIR` in `.env` (default: `/media/devmon/docker-disk/hermes-data/workspace`).
3. Put `token.json` and `credentials.json` into `./data/`.

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
