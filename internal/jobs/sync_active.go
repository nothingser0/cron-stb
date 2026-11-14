package jobs

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"cron-stb/internal/config"
	"cron-stb/internal/discord"
	"cron-stb/internal/sheets"
)

type SyncActiveJob struct{}

func init() {
	Register(&SyncActiveJob{})
}

func (j *SyncActiveJob) Name() string {
	return "sync_active"
}

func (j *SyncActiveJob) Description() string {
	return "Sync active airdrop tasks from Google Sheets to Discord"
}

func (j *SyncActiveJob) Schedule() string {
	return config.GetString("SYNC_ACTIVE_SCHEDULE", "0 8 * * *")
}

func (j *SyncActiveJob) Run(ctx context.Context) error {
	cfg := config.LoadConfig()
	isDryRun, _ := ctx.Value(DryRunKey).(bool)

	token := cfg.GetJobDiscordToken("SYNC_ACTIVE_DISCORD_TOKEN")
	if token == "" && !isDryRun {
		return fmt.Errorf("missing DISCORD_BOT_TOKEN")
	}

	if cfg.TokenFile == "" {
		return fmt.Errorf("token.json not found")
	}

	sheetID := cfg.GetJobSheetID("SYNC_ACTIVE_SHEET_ID")
	if sheetID == "" {
		return fmt.Errorf("SHEET_ID not configured")
	}

	channelID := config.GetInt64("SYNC_ACTIVE_CHANNEL_ID", 0)
	if channelID == 0 && !isDryRun {
		return fmt.Errorf("SYNC_ACTIVE_CHANNEL_ID not configured")
	}

	dailySheet := config.GetString("SYNC_ACTIVE_SHEET_DAILY", "Daily")
	todaySheet := config.GetString("SYNC_ACTIVE_SHEET_TODAY", "Today")

	client, err := sheets.NewClient(ctx, cfg.TokenFile, cfg.CredentialsFile)
	if err != nil {
		return fmt.Errorf("sheets client: %w", err)
	}

	dailyRows, err := client.ReadSheet(sheetID, dailySheet)
	if err != nil {
		return fmt.Errorf("read %s: %w", dailySheet, err)
	}

	todayRows, _ := client.ReadSheet(sheetID, todaySheet)

	message := formatActiveTasks(dailyRows, todayRows)
	if message == "" {
		return fmt.Errorf("formatted message is empty")
	}

	if isDryRun {
		log.Printf("[sync_active] dry-run preview (channel: %d):", channelID)
		fmt.Println(message)
		return nil
	}

	discordClient := discord.NewClient(token)
	if err := discordClient.SendChannelMessage(channelID, message); err != nil {
		return fmt.Errorf("send discord: %w", err)
	}

	log.Printf("[sync_active] posted to channel %d", channelID)
	return nil
}

func formatActiveTasks(dailyData, todayData [][]string) string {
	var dailyTasks []string
	for i, row := range dailyData {
		if i < 2 || len(row) < 32 {
			continue
		}
		if name := strings.TrimSpace(row[31]); name != "" {
			dailyTasks = append(dailyTasks, "• "+name)
		}
	}

	var todayTasks []string
	for i, row := range todayData {
		if i < 6 || len(row) < 32 {
			continue
		}
		if name := strings.TrimSpace(row[31]); name != "" {
			todayTasks = append(todayTasks, "• "+name)
		}
	}

	total := len(dailyTasks) + len(todayTasks)

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	now := time.Now().In(loc)

	var lines []string
	lines = append(lines, fmt.Sprintf("🚀 **Garapan Airdrop Hari Ini** — %s", now.Format("02 Jan 2006")), "")

	if len(todayTasks) > 0 {
		lines = append(lines, "🔥 **Fokus Hari Ini (Today):**")
		limit := min(len(todayTasks), 20)
		lines = append(lines, todayTasks[:limit]...)
		if len(todayTasks) > 20 {
			lines = append(lines, fmt.Sprintf("_...dan %d garapan lainnya_", len(todayTasks)-20))
		}
		lines = append(lines, "")
	}

	if len(dailyTasks) > 0 {
		lines = append(lines, "📅 **Garapan Rutin (Daily):**")
		limit := min(len(dailyTasks), 20)
		lines = append(lines, dailyTasks[:limit]...)
		if len(dailyTasks) > 20 {
			lines = append(lines, fmt.Sprintf("_...dan %d garapan lainnya_", len(dailyTasks)-20))
		}
		lines = append(lines, "")
	}

	if total == 0 {
		lines = append(lines,
			"Tidak ada garapan airdrop aktif untuk hari ini. 🎉",
			"✨ _Semua garapan sudah beres atau belum ada jadwal baru._",
		)
	} else {
		lines = append(lines, fmt.Sprintf("📊 _Total %d garapan aktif hari ini._", total))
	}

	return strings.Join(lines, "\n")
}
