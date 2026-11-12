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

type ScheduleDailyJob struct{}

func init() {
	Register(&ScheduleDailyJob{})
}

func (j *ScheduleDailyJob) Name() string {
	return "schedule_daily"
}

func (j *ScheduleDailyJob) Description() string {
	return "Morning schedule, events, and deadline briefing to Discord"
}

func (j *ScheduleDailyJob) Schedule() string {
	return config.GetString("SCHEDULE_SCHEDULE", "0 7 * * *")
}

type scheduleEntry struct {
	ID        string `json:"id"`
	Judul     string `json:"judul"`
	Title     string `json:"title"`
	Event     string `json:"event"`
	Tipe      string `json:"tipe"`
	Type      string `json:"type"`
	Tanggal   string `json:"tanggal"`
	Date      string `json:"date"`
	Waktu     string `json:"waktu"`
	Time      string `json:"time"`
	Lokasi    string `json:"lokasi"`
	Location  string `json:"location"`
	Catatan   string `json:"catatan"`
	Pengingat string `json:"pengingat"`
	Status    string `json:"status"`
}

func (e *scheduleEntry) GetJudul() string {
	if e.Judul != "" {
		return e.Judul
	}
	if e.Title != "" {
		return e.Title
	}
	return e.Event
}

func (e *scheduleEntry) GetTipe() string {
	if e.Tipe != "" {
		return strings.ToLower(e.Tipe)
	}
	return strings.ToLower(e.Type)
}

func (e *scheduleEntry) GetWaktu() string {
	if e.Waktu != "" {
		return e.Waktu
	}
	return e.Time
}

func (e *scheduleEntry) GetLokasi() string {
	if e.Lokasi != "" {
		return e.Lokasi
	}
	return e.Location
}

func (j *ScheduleDailyJob) Run(ctx context.Context) error {
	cfg := config.LoadConfig()
	isDryRun, _ := ctx.Value(DryRunKey).(bool)

	token := cfg.GetJobDiscordToken("SCHEDULE_DISCORD_TOKEN")
	if token == "" && !isDryRun {
		return fmt.Errorf("missing DISCORD_BOT_TOKEN")
	}

	channelID := config.GetInt64("SCHEDULE_CHANNEL_ID", 1511383864818274305)
	if channelID == 0 && !isDryRun {
		return fmt.Errorf("SCHEDULE_CHANNEL_ID not configured")
	}

	now := getWIBNow()
	todayStr := now.Format("2006-01-02")
	filename := fmt.Sprintf("jadwal-%s.jsonl", todayStr)

	filePath := findDataFile("schedule", filename)
	var entries []scheduleEntry

	if filePath != "" {
		lines, err := readLines(filePath)
		if err == nil {
			for _, line := range lines {
				var e scheduleEntry
				if err := json.Unmarshal([]byte(line), &e); err == nil {
					st := strings.ToLower(e.Status)
					if st == "aktif" || st == "" {
						entries = append(entries, e)
					}
				}
			}
		}
	}

	var events, deadlines, reminders []scheduleEntry
	for _, e := range entries {
		switch e.GetTipe() {
		case "deadline":
			deadlines = append(deadlines, e)
		case "reminder":
			reminders = append(reminders, e)
		default:
			events = append(events, e)
		}
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("🌅 **Agenda & Jadwal Hari Ini** — %s", now.Format("Monday, 02 Jan 2006")), "")

	if len(entries) == 0 {
		lines = append(lines,
			"Tidak ada jadwal atau deadline khusus yang tercatat untuk hari ini.",
			"✨ _Waktu fokus bebas atau istirahat! Tambah jadwal kapan saja dengan mention bot._",
		)
	} else {
		if len(events) > 0 {
			lines = append(lines, "📅 **Event / Pertemuan:**")
			for _, e := range events {
				timeStr := e.GetWaktu()
				if timeStr == "" {
					timeStr = "Waktu bebas"
				}
				locStr := ""
				if loc := e.GetLokasi(); loc != "" {
					locStr = fmt.Sprintf(" (📍 %s)", loc)
				}
				lines = append(lines, fmt.Sprintf("• `%s` **%s**%s", timeStr, e.GetJudul(), locStr))
			}
			lines = append(lines, "")
		}

		if len(deadlines) > 0 {
			lines = append(lines, "⏰ **Deadline Hari Ini:**")
			for _, d := range deadlines {
				timeStr := ""
				if t := d.GetWaktu(); t != "" {
					timeStr = fmt.Sprintf(" [%s]", t)
				}
				lines = append(lines, fmt.Sprintf("• ⚠️ **%s**%s", d.GetJudul(), timeStr))
			}
			lines = append(lines, "")
		}

		if len(reminders) > 0 {
			lines = append(lines, "🔔 **Reminder:**")
			for _, r := range reminders {
				lines = append(lines, fmt.Sprintf("• %s", r.GetJudul()))
			}
			lines = append(lines, "")
		}

		lines = append(lines, "💪 _Semoga harimu produktif dan lancar!_")
	}

	message := strings.Join(lines, "\n")

	if isDryRun {
		log.Printf("[schedule_daily] dry-run preview (channel: %d):", channelID)
		fmt.Println(message)
		return nil
	}

	discordClient := discord.NewClient(token)
	if err := discordClient.SendChannelMessage(channelID, message); err != nil {
		return fmt.Errorf("send discord: %w", err)
	}

	log.Printf("[schedule_daily] posted daily schedule to channel %d", channelID)
	return nil
}
