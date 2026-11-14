package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"cron-stb/internal/config"
	"cron-stb/internal/discord"
)

type GoalsWeeklyJob struct{}

func init() {
	Register(&GoalsWeeklyJob{})
}

func (j *GoalsWeeklyJob) Name() string {
	return "goals_weekly"
}

func (j *GoalsWeeklyJob) Description() string {
	return "Weekly personal goals check-in & milestone review to Discord"
}

func (j *GoalsWeeklyJob) Schedule() string {
	return config.GetString("GOALS_SCHEDULE", "0 9 * * 1")
}

type goalEntry struct {
	ID        string `json:"id"`
	Goal      string `json:"goal"`
	Kategori  string `json:"kategori"`
	Ukuran    string `json:"ukuran"`
	Deadline  string `json:"deadline"`
	Status    string `json:"status"` // aktif | tercapai | ditunda | dilepas
}

func (j *GoalsWeeklyJob) Run(ctx context.Context) error {
	cfg := config.LoadConfig()
	isDryRun, _ := ctx.Value(DryRunKey).(bool)

	token := cfg.GetJobDiscordToken("GOALS_DISCORD_TOKEN")
	if token == "" && !isDryRun {
		return fmt.Errorf("missing DISCORD_BOT_TOKEN")
	}

	channelID := config.GetInt64("GOALS_CHANNEL_ID", 0)
	if channelID == 0 && !isDryRun {
		return fmt.Errorf("GOALS_CHANNEL_ID not configured")
	}

	now := getWIBNow()
	files := findDataGlob("personal_goals", "goals-*.jsonl")
	var activeGoals []goalEntry

	for _, f := range files {
		lines, err := readLines(f)
		if err != nil {
			continue
		}
		for _, line := range lines {
			var g goalEntry
			if err := json.Unmarshal([]byte(line), &g); err == nil {
				st := strings.ToLower(g.Status)
				if st == "aktif" || st == "" {
					activeGoals = append(activeGoals, g)
				}
			}
		}
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("🎯 **Weekly Goals Check-in** — Minggu Ini (%s)", now.Format("02 Jan 2006")), "")

	if len(activeGoals) == 0 {
		lines = append(lines,
			"Belum ada target (goals) aktif yang tercatat.",
			"💡 _Awali minggu dengan mencatat 1 target terukur di channel ini (contoh: `Target lari 10K sebelum 31 Des`)._",
		)
	} else {
		lines = append(lines, "📌 **Target Aktif:**")
		for _, g := range activeGoals {
			kat := strings.ToUpper(g.Kategori)
			if kat == "" {
				kat = "UMUM"
			}
			dl := g.Deadline
			if dl == "" {
				dl = "Tanpa batas waktu"
			}
			ukur := ""
			if g.Ukuran != "" {
				ukur = fmt.Sprintf("\n   ↳ *Tolok ukur:* %s", g.Ukuran)
			}
			lines = append(lines, fmt.Sprintf("• [%s] %s (Deadline: %s)%s", kat, g.Goal, dl, ukur))
		}
		lines = append(lines, "", "🔥 _Apa satu langkah kecil yang akan kamu eksekusi minggu ini untuk mendekatkan targetmu?_")
	}

	message := strings.Join(lines, "\n")

	if isDryRun {
		log.Printf("[goals_weekly] dry-run preview (channel: %d):", channelID)
		fmt.Println(message)
		return nil
	}

	discordClient := discord.NewClient(token)
	if err := discordClient.SendChannelMessage(channelID, message); err != nil {
		return fmt.Errorf("send discord: %w", err)
	}

	log.Printf("[goals_weekly] posted weekly check-in to channel %d", channelID)
	return nil
}
