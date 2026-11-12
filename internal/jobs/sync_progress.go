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
		message = "📊 **Airdrop Progress Update**\n\nNo data available."
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
	totalGarap := 0

	for i, row := range sheetsData {
		if i < 6 || len(row) < 34 {
			continue
		}

		status := strings.ToLower(strings.TrimSpace(row[0]))
		nama := strings.TrimSpace(row[33])

		if len(row) > 3 && strings.TrimSpace(row[3]) != "" {
			totalGarap++
		}

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
		fmt.Sprintf("**Airdrop Progress Update** — %s", now.Format("02 Jan 2006")),
		"",
	)

	if totalGarap > 0 {
		lines = append(lines,
			fmt.Sprintf("📊 **Total:** %d airdrops", totalGarap),
			"",
			"**Summary:**",
			fmt.Sprintf("• Win: %d", len(winList)),
			fmt.Sprintf("• Waiting: %d", len(waitingList)),
			fmt.Sprintf("• Lose: %d", len(loseList)),
			fmt.Sprintf("• Received: %d", len(receivedList)),
			"",
		)
	}

	appendSection := func(title string, list []string, limit int) {
		if len(list) == 0 {
			return
		}
		lines = append(lines, title)
		take := min(len(list), limit)
		lines = append(lines, list[:take]...)
		if len(list) > limit {
			lines = append(lines, fmt.Sprintf("_...and %d more_", len(list)-limit))
		}
		lines = append(lines, "")
	}

	appendSection("⏳ **Waiting:**", waitingList, 15)
	appendSection("🎉 **Win:**", winList, 15)
	appendSection("❌ **Lose:**", loseList, 10)
	appendSection("💰 **Received:**", receivedList, 15)

	return strings.Join(lines, "\n")
}
