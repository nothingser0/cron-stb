# ⏱️ Cron STB

**Cron STB** is an ultra-lightweight, standalone cron scheduler and Discord notification daemon engineered in **Golang** for 24/7 operation on low-resource devices (Armbian TV boxes / STB, Raspberry Pi, CasaOS, and low-memory VPS).

It automates two primary workflows in a single self-contained binary:
1. **Google Sheets Airdrop Automation**: Tracks active tasks, monitors Win/Waiting/Received/Lose progress, sends H-1 deadline alerts, and performs monthly sheet archiving.
2. **Hermes Personal Workspace Briefings**: Reads local JSONL logs (`schedule`, `tasks`, `finance`, `personal_goals`, `learning`, `personal_journal`) and dispatches structured daily, weekly, and monthly briefings to dedicated Discord channels.

---

## ✨ Key Features

- **Ultra-Low Memory Footprint (Golang)**: Compiled as a single static binary (`CGO_ENABLED=0`) with minimal dependencies — idles at **`~8–12 MB`** RAM.
- **Multi-Arch Docker & GHCR Publishing**: Prebuilt multi-arch Docker images (`linux/amd64` & `linux/arm64`) automatically built and published to GitHub Container Registry (`ghcr.io`) via GitHub Actions.
- **11 Built-in Scheduled Jobs**:
  - **Airdrop Tracking (Google Sheets API v4)**: `sync_progress`, `sync_active`, `alert_airdrop_h1`, `archive_monthly`.
  - **Personal Productivity & Finance (JSONL)**: `schedule_daily`, `tasks_daily`, `finance_daily`, `finance_monthly`, `goals_weekly`, `learning_weekly`, `journal_nightly`.
- **Smart Discord Message Splitting**: Automatically splits long reports into safe chunks (`<= 1900` chars) with rate-limit pacing.
- **CLI Inspection & Dry-Run Mode**: Test and preview any job's formatted Discord output directly in the terminal (`-dry-run`) without sending live messages.
- **CasaOS Ready**: Includes `x-casaos` metadata in `docker-compose.yml` for one-click import on CasaOS dashboards.

---

## 📅 Registered Jobs

### 1. Airdrop Automation (Google Sheets)
| Job Name | Default Schedule (WIB) | Target Channel Env | Description |
| :--- | :--- | :--- | :--- |
| `sync_progress` | `0 6 * * *` (06:00) | `SYNC_PROGRESS_CHANNEL_ID` | Rekap status garapan (Win, Waiting, Received, Lose) & Win Rate |
| `sync_active` | `0 8 * * *` (08:00) | `SYNC_ACTIVE_CHANNEL_ID` | Daftar garapan fokus hari ini (Today) & garapan rutin (Daily) |
| `archive_monthly` | `0 18 * * *` (18:00)* | `ARCHIVE_CHANNEL_ID` | Memindahkan baris `Received` & `Lose` dari *Already Garap* ke *Trash* (tgl 1 / akhir bulan) |
| `alert_airdrop_h1` | `0 21 * * *` (21:00) | `ALERT_H1_CHANNEL_ID` | Alert garapan yang berakhir hari ini & besok (H-1) diurutkan dari reward terbesar |

### 2. Personal & Workspace Briefings (JSONL)
| Job Name | Default Schedule (WIB) | Target Channel Env | Description |
| :--- | :--- | :--- | :--- |
| `schedule_daily` | `0 7 * * *` (07:00) | `SCHEDULE_CHANNEL_ID` | Briefing agenda, pertemuan, deadline & reminder pagi |
| `tasks_daily` | `0 8 * * *` (08:00) | `TASK_CHANNEL_ID` | Morning task briefing, task jatuh tempo & 3 rekomendasi fokus |
| `goals_weekly` | `0 9 * * 1` (Senin 09:00) | `GOALS_CHANNEL_ID` | Check-in mingguan target aktif & tolok ukur |
| `learning_weekly` | `0 19 * * 0` (Minggu 19:00) | `LEARNING_CHANNEL_ID` | Rekap milestone belajar, log sesi 7 hari terakhir & materi aktif |
| `journal_nightly` | `0 21 * * *` (21:00) | `JOURNAL_CHANNEL_ID` | Pengingat refleksi jurnal malam |
| `finance_daily` | `0 22 * * *` (22:00) | `FINANCE_CHANNEL_ID` | Rekap keuangan harian (hanya dikirim jika ada transaksi tercatat hari itu) |
| `finance_monthly` | `0 8 1 * *` (Tgl 1, 08:00) | `FINANCE_CHANNEL_ID` | Rekap bulanan, arus kas bersih, rata-rata harian & top 5 pengeluaran |

---

## 🛠️ System Requirements

- **Docker & Docker Compose** (recommended for 24/7 Armbian STB / CasaOS / Raspberry Pi deployment) **or**
- **Go 1.25+** (for bare-metal execution)

---

## 🚀 Quick Start

### Option 1: Docker Compose (Recommended for STB / CasaOS)

Prebuilt multi-arch images (`linux/amd64` & `linux/arm64`) are published to `ghcr.io/nothingser0/cron-stb:latest`.

1. **Clone the repository**:
   ```bash
   git clone https://github.com/nothingser0/cron-stb.git
   cd cron-stb
   ```

2. **Configure environment variables & Google OAuth files**:
   ```bash
   cp .env.example .env
   nano .env
   ```
   Place your Google OAuth `credentials.json` and `token.json` inside `./data/`.

3. **Pull prebuilt image & start in background** *(saves STB CPU & RAM)*:
   ```bash
   docker compose pull
   docker compose up -d
   ```
   *(Or build locally from source with `docker compose up -d --build`)*

4. **Check logs**:
   ```bash
   docker compose logs -f cron-stb
   ```

---

### Option 2: Native Go Execution (Bare-Metal / CLI)

1. **Build the binary**:
   ```bash
   go build -buildvcs=false -ldflags="-s -w" -o cron-stb ./cmd/cron-stb
   ```

2. **CLI Commands**:
   ```bash
   # Start the background cron scheduler daemon (default)
   ./cron-stb daemon

   # List all registered jobs and their active schedules
   ./cron-stb list

   # Check detected configuration, paths, and token status
   ./cron-stb config

   # Preview a job's Discord message in terminal without sending (Dry-Run)
   ./cron-stb -dry-run run schedule_daily
   ./cron-stb -dry-run run alert_airdrop_h1

   # Manually trigger and send a single job to Discord immediately
   ./cron-stb run sync_active
   ```

---

## ⚙️ Environment Configuration (`.env`)

| Variable | Description | Default / Example |
| :--- | :--- | :--- |
| `DISCORD_BOT_TOKEN` | Primary Discord Bot Token (`Bot ...`) | `your_discord_bot_token` |
| `SHEET_ID` | Google Spreadsheet ID for Airdrop jobs | `1AbCdEfGhIjKlMnOpQrStUvWxYz` |
| `DATA_DIR` | Directory containing `token.json` & `credentials.json` | `data` |
| `HERMES_WORKSPACE_DIR` | Path to Hermes workspace containing JSONL data folders | `/media/devmon/docker-disk/hermes-data/workspace` |
| `SYNC_ACTIVE_CHANNEL_ID` | Discord Channel ID for `sync_active` | `123456789012345678` |
| `SYNC_PROGRESS_CHANNEL_ID` | Discord Channel ID for `sync_progress` | `123456789012345678` |
| `ALERT_H1_CHANNEL_ID` | Discord Channel ID for `alert_airdrop_h1` | `123456789012345678` |
| `ARCHIVE_CHANNEL_ID` | Discord Channel ID for `archive_monthly` | `123456789012345678` |
| `SCHEDULE_CHANNEL_ID` | Discord Channel ID for `schedule_daily` | `123456789012345678` |
| `TASK_CHANNEL_ID` | Discord Channel ID for `tasks_daily` | `123456789012345678` |
| `FINANCE_CHANNEL_ID` | Discord Channel ID for `finance_daily` & `finance_monthly` | `123456789012345678` |
| `GOALS_CHANNEL_ID` | Discord Channel ID for `goals_weekly` | `123456789012345678` |
| `LEARNING_CHANNEL_ID` | Discord Channel ID for `learning_weekly` | `123456789012345678` |
| `JOURNAL_CHANNEL_ID` | Discord Channel ID for `journal_nightly` | `123456789012345678` |

*(Each job also supports overriding its cron expression via `*_SCHEDULE` and sheet tab names via `*_SHEET_*` in `.env` — see [.env.example](.env.example) for the full list).*
