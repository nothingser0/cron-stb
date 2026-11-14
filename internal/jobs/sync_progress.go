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

type SyncProgressJob struct{}

func init() {
	Register(&SyncProgressJob{})
}

func (j *SyncProgressJob) Name() string {
	return "sync_progress"
}

func (j *SyncProgressJob) Description() string {
	return "Sync progress update (Win/Waiting/Lose/Received) from Google Sheets"
}

func (j *SyncProgressJob) Schedule() string {
	return config.GetString("SYNC_PROGRESS_SCHEDULE", "0 6 * * *")
}

func (j *SyncProgressJob) Run(ctx context.Context) error {
	cfg := config.LoadConfig()
	isDryRun, _ := ctx.Value(DryRunKey).(bool)

	token := cfg.GetJobDiscordToken("SYNC_PROGRESS_DISCORD_TOKEN")
	if token == "" && !isDryRun {
		return fmt.Errorf("missing DISCORD_BOT_TOKEN")
	}

	if cfg.TokenFile == "" {
		return fmt.Errorf("token.json not found")
	}

	sheetID := cfg.GetJobSheetID("SYNC_PROGRESS_SHEET_ID")
	if sheetID == "" {
		return fmt.Errorf("SHEET_ID not configured")
	}

	channelID := config.GetInt64("SYNC_PROGRESS_CHANNEL_ID", 0)
	if channelID == 0 && !isDryRun {
		return fmt.Errorf("SYNC_PROGRESS_CHANNEL_ID not configured")
	}

	sheetName := config.GetString("SYNC_PROGRESS_SHEET_NAME", "Trash")

	client, err := sheets.NewClient(ctx, cfg.TokenFile, cfg.CredentialsFile)
	if err != nil {
		return fmt.Errorf("sheets client: %w", err)
	}

	rows, err := client.ReadSheet(sheetID, sheetName)
	if err != nil {
		return fmt.Errorf("read %s: %w", sheetName, err)
	}

	message := formatProgress(rows)

	if isDryRun {
		log.Printf("[sync_progress] dry-run preview (channel: %d):", channelID)
		if message != "" {
			fmt.Println(message)
		} else {
			fmt.Println("No progress data available.")
		}
		return nil
	}

	if message == "" {
		now := getWIBNow()
		message = fmt.Sprintf("📈 **Rekap Progress Airdrop** — %s\n\nBelum ada data progress yang tercatat.\n💡 _Update status garapan di Google Sheets untuk melihat rekap di sini._", now.Format("02 Jan 2006"))
	}

	discordClient := discord.NewClient(token)
	if err := discordClient.SendChannelMessage(channelID, message); err != nil {
		return fmt.Errorf("send discord: %w", err)
	}

	log.Printf("[sync_progress] posted to channel %d", channelID)
	return nil
}

func formatProgress(sheetsData [][]string) string {
	if len(sheetsData) == 0 {
		return ""
	}

	var winList, waitingList, loseList, receivedList []string

	for i, row := range sheetsData {
		if i < 6 || len(row) < 34 {
			continue
		}

		status := strings.ToLower(strings.TrimSpace(row[0]))
		nama := strings.TrimSpace(row[33])

		if status == "" || nama == "" {
			continue
		}

		item := "• " + nama
		switch status {
		case "win":
			winList = append(winList, item)
		case "waiting":
			waitingList = append(waitingList, item)
		case "lose":
			loseList = append(loseList, item)
		case "received":
			receivedList = append(receivedList, item)
		}
	}

	if len(winList) == 0 && len(waitingList) == 0 && len(loseList) == 0 && len(receivedList) == 0 {
		return ""
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	now := time.Now().In(loc)

	var lines []string
	lines = append(lines,
		fmt.Sprintf("📈 **Rekap Progress Airdrop** — %s", now.Format("02 Jan 2006")),
		"",
	)

	appendSection := func(title string, list []string, limit int) {
		if len(list) == 0 {
			return
		}
		lines = append(lines, fmt.Sprintf("%s (%d):", title, len(list)))
		take := min(len(list), limit)
		lines = append(lines, list[:take]...)
		if len(list) > limit {
			lines = append(lines, fmt.Sprintf("_...dan %d lainnya_", len(list)-limit))
		}
		lines = append(lines, "")
	}

	appendSection("🎉 **Win / Siap Claim**", winList, 15)
	appendSection("⏳ **Waiting**", waitingList, 15)
	appendSection("💰 **Received / Cair**", receivedList, 15)
	appendSection("❌ **Lose**", loseList, 10)

	totalTracked := len(winList) + len(waitingList) + len(receivedList) + len(loseList)
	resolved := len(winList) + len(receivedList) + len(loseList)
	winRateStr := ""
	if resolved > 0 {
		winRate := float64(len(winList)+len(receivedList)) / float64(resolved) * 100
		winRateStr = fmt.Sprintf(" • Win Rate: %.0f%%", winRate)
	}

	lines = append(lines, fmt.Sprintf("📊 _Total %d garapan dipantau%s._", totalTracked, winRateStr))

	return strings.Join(lines, "\n")
}
