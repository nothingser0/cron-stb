package jobs

import (
	"context"
	"fmt"
	"log"
	"strings"

	"cron-stb/internal/config"
	"cron-stb/internal/discord"
)

type JournalNightlyJob struct{}

func init() {
	Register(&JournalNightlyJob{})
}

func (j *JournalNightlyJob) Name() string {
	return "journal_nightly"
}

func (j *JournalNightlyJob) Description() string {
	return "Nightly personal journal reflection prompt to Discord"
}

func (j *JournalNightlyJob) Schedule() string {
	return config.GetString("JOURNAL_SCHEDULE", "0 21 * * *")
}

func (j *JournalNightlyJob) Run(ctx context.Context) error {
	cfg := config.LoadConfig()
	isDryRun, _ := ctx.Value(DryRunKey).(bool)

	token := cfg.GetJobDiscordToken("JOURNAL_DISCORD_TOKEN")
	if token == "" && !isDryRun {
		return fmt.Errorf("missing DISCORD_BOT_TOKEN")
	}

	channelID := config.GetInt64("JOURNAL_CHANNEL_ID", 1511383970426654874)
	if channelID == 0 && !isDryRun {
		return fmt.Errorf("JOURNAL_CHANNEL_ID not configured")
	}

	now := getWIBNow()
	todayStr := now.Format("2006-01-02")
	filename := fmt.Sprintf("jurnal-%s.jsonl", todayStr)

	filePath := findDataFile("personal_journal", filename)
	alreadyWritten := false

	if filePath != "" {
		lines, err := readLines(filePath)
		if err == nil && len(lines) > 0 {
			alreadyWritten = true
		}
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("🌙 **Refleksi Malam** — %s", now.Format("02 Jan 2006")), "")

	if alreadyWritten {
		lines = append(lines,
			"Catatan jurnal harimu sudah tersimpan rapi.",
			"Lepaskan penat hari ini dan selamat beristirahat yang nyenyak. Sampai jumpa besok pagi! 🌿",
		)
	} else {
		lines = append(lines,
			"Bagaimana harimu hari ini?",
			"Jika ada peristiwa berkesan, pikiran yang mengganjal, atau perasaan yang ingin kamu simpan, ruang ini selalu aman untuk mendengarkan.",
			"",
			"✍️ _Tulis bebas apa pun yang kamu rasakan sebelum menutup hari._",
		)
	}

	message := strings.Join(lines, "\n")

	if isDryRun {
		log.Printf("[journal_nightly] dry-run preview (channel: %d):", channelID)
		fmt.Println(message)
		return nil
	}

	discordClient := discord.NewClient(token)
	if err := discordClient.SendChannelMessage(channelID, message); err != nil {
		return fmt.Errorf("send discord: %w", err)
	}

	log.Printf("[journal_nightly] posted reflection reminder to channel %d", channelID)
	return nil
}
