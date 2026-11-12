package jobs

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"cron-stb/internal/config"
	"cron-stb/internal/discord"
	"cron-stb/internal/sheets"
)

type ArchiveMonthlyJob struct{}

func init() {
	Register(&ArchiveMonthlyJob{})
}

func (j *ArchiveMonthlyJob) Name() string {
	return "archive_monthly"
}

func (j *ArchiveMonthlyJob) Description() string {
	return "Monthly archive: Move Received + Lose from Already Garap to Trash"
}

func (j *ArchiveMonthlyJob) Schedule() string {
	return config.GetString("ARCHIVE_SCHEDULE", "0 18 * * *")
}

func isMonthlyTriggerWIB() bool {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	today := time.Now().In(loc)
	tomorrow := today.AddDate(0, 0, 1)

	return today.Day() == 1 || tomorrow.Month() != today.Month()
}

func (j *ArchiveMonthlyJob) Run(ctx context.Context) error {
	isManual, _ := ctx.Value(ManualRunKey).(bool)
	force := strings.ToLower(os.Getenv("FORCE_ARCHIVE"))
	isForce := force == "1" || force == "true" || force == "yes" || isManual

	if !isForce && !isMonthlyTriggerWIB() {
		log.Println("[archive_monthly] skipping (runs only on 1st or last day of month)")
		return nil
	}

	cfg := config.LoadConfig()
	token := cfg.GetJobDiscordToken("ARCHIVE_DISCORD_TOKEN")
	if token == "" {
		return fmt.Errorf("missing DISCORD_BOT_TOKEN")
	}

	tokenPath := cfg.TokenWriteFile
	if tokenPath == "" {
		tokenPath = cfg.TokenFile
	}
	if tokenPath == "" {
		return fmt.Errorf("token file not found")
	}

	sheetID := cfg.GetJobSheetID("ARCHIVE_SHEET_ID")
	if sheetID == "" {
		return fmt.Errorf("SHEET_ID not configured")
	}

	channelID := config.GetInt64("ARCHIVE_CHANNEL_ID", 0)
	if channelID == 0 {
		return fmt.Errorf("ARCHIVE_CHANNEL_ID not configured")
	}

	sourceSheet := config.GetString("ARCHIVE_SHEET_SOURCE", "Already Garap")
	destSheet := config.GetString("ARCHIVE_SHEET_DEST", "Trash")

	client, err := sheets.NewClient(ctx, tokenPath, cfg.CredentialsFile)
	if err != nil {
		return fmt.Errorf("sheets client: %w", err)
	}

	allData, err := client.ReadSheet(sheetID, sourceSheet)
	if err != nil {
		return fmt.Errorf("read %s: %w", sourceSheet, err)
	}

	if len(allData) < 7 {
		log.Printf("[archive_monthly] %s has no data rows", sourceSheet)
		return nil
	}

	header := allData[:6]
	dataRows := allData[6:]

	var toArchive [][]string
	var toKeep [][]string
	var receivedList, loseList []string
	receivedCount, loseCount := 0, 0

	for _, row := range dataRows {
		if len(row) == 0 {
			continue
		}

		status := strings.ToLower(strings.TrimSpace(row[0]))
		name := "N/A"
		if len(row) > 33 && strings.TrimSpace(row[33]) != "" {
			name = strings.TrimSpace(row[33])
		}

		if status == "received" || status == "lose" {
			toArchive = append(toArchive, row)
			if status == "received" {
				receivedCount++
				receivedList = append(receivedList, "• "+name)
			} else {
				loseCount++
				loseList = append(loseList, "• "+name)
			}
		} else {
			toKeep = append(toKeep, row)
		}
	}

	if len(toArchive) == 0 {
		log.Println("[archive_monthly] no rows to archive")
		return nil
	}

	destData, err := client.ReadSheet(sheetID, destSheet)
	if err != nil {
		return fmt.Errorf("read %s: %w", destSheet, err)
	}

	firstEmptyRow := len(destData) + 1
	for i := 6; i < len(destData); i++ {
		if len(destData[i]) == 0 || strings.TrimSpace(destData[i][0]) == "" {
			firstEmptyRow = i + 1
			break
		}
	}

	endRow := firstEmptyRow + len(toArchive) - 1
	destRange := fmt.Sprintf("%s!A%d:%d", destSheet, firstEmptyRow, endRow)

	if err := client.UpdateSheet(sheetID, destRange, toArchive); err != nil {
		return fmt.Errorf("append to %s: %w", destSheet, err)
	}

	if err := client.ClearSheet(sheetID, sourceSheet); err != nil {
		return fmt.Errorf("clear %s: %w", sourceSheet, err)
	}

	newData := append(header, toKeep...)
	if err := client.UpdateSheet(sheetID, fmt.Sprintf("%s!A1", sourceSheet), newData); err != nil {
		return fmt.Errorf("update %s: %w", sourceSheet, err)
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	monthName := time.Now().In(loc).Format("January 2006")

	var lines []string
	lines = append(lines,
		fmt.Sprintf("📦 **Airdrop Archive** — %s", monthName),
		"",
		fmt.Sprintf("Total archived: %d projects", len(toArchive)),
		fmt.Sprintf("Received: %d", receivedCount),
		fmt.Sprintf("Lose: %d", loseCount),
		"",
		"**Received:**",
	)
	if len(receivedList) > 0 {
		lines = append(lines, receivedList...)
	} else {
		lines = append(lines, "• None")
	}

	lines = append(lines, "", "**Lose:**")
	if len(loseList) > 0 {
		lines = append(lines, loseList...)
	} else {
		lines = append(lines, "• None")
	}

	lines = append(lines, "", fmt.Sprintf("Moved to: Google Sheets %s", destSheet))

	discordClient := discord.NewClient(token)
	if err := discordClient.SendChannelMessage(channelID, strings.Join(lines, "\n")); err != nil {
		return fmt.Errorf("send discord: %w", err)
	}

	log.Printf("[archive_monthly] archived %d rows and posted summary", len(toArchive))
	return nil
}
