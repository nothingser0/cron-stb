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

type LearningWeeklyJob struct{}

func init() {
	Register(&LearningWeeklyJob{})
}

func (j *LearningWeeklyJob) Name() string {
	return "learning_weekly"
}

func (j *LearningWeeklyJob) Description() string {
	return "Weekly learning progress & active resources review to Discord"
}

func (j *LearningWeeklyJob) Schedule() string {
	return config.GetString("LEARNING_SCHEDULE", "0 19 * * 0")
}

type progressEntry struct {
	ID        string `json:"id"`
	Tanggal   string `json:"tanggal"`
	Jenis     string `json:"jenis"` // log | milestone
	Topik     string `json:"topik"`
	Kegiatan  string `json:"kegiatan"`
	Capaian   string `json:"capaian"`
	Kendala   string `json:"kendala"`
}

type materialEntry struct {
	ID        string `json:"id"`
	Judul     string `json:"judul"`
	Topik     string `json:"topik"`
	Status    string `json:"status"` // belum | sedang | selesai
}

func (j *LearningWeeklyJob) Run(ctx context.Context) error {
	cfg := config.LoadConfig()
	isDryRun, _ := ctx.Value(DryRunKey).(bool)

	token := cfg.GetJobDiscordToken("LEARNING_DISCORD_TOKEN")
	if token == "" && !isDryRun {
		return fmt.Errorf("missing DISCORD_BOT_TOKEN")
	}

	channelID := config.GetInt64("LEARNING_CHANNEL_ID", 0)
	if channelID == 0 && !isDryRun {
		return fmt.Errorf("LEARNING_CHANNEL_ID not configured")
	}

	now := getWIBNow()
	weekAgo := now.AddDate(0, 0, -7)

	progressFiles := findDataGlob("learning_milestones", "progress-*.jsonl")
	var recentLogs []progressEntry
	var milestones []progressEntry

	for _, f := range progressFiles {
		lines, err := readLines(f)
		if err != nil {
			continue
		}
		for _, line := range lines {
			var p progressEntry
			if err := json.Unmarshal([]byte(line), &p); err == nil {
				if p.Tanggal >= weekAgo.Format("2006-01-02") {
					if strings.ToLower(p.Jenis) == "milestone" {
						milestones = append(milestones, p)
					} else {
						recentLogs = append(recentLogs, p)
					}
				}
			}
		}
	}

	materialFiles := findDataGlob("learning_materials", "materi-*.jsonl")
	var ongoingMaterials []materialEntry

	for _, f := range materialFiles {
		lines, err := readLines(f)
		if err != nil {
			continue
		}
		for _, line := range lines {
			var m materialEntry
			if err := json.Unmarshal([]byte(line), &m); err == nil {
				if strings.ToLower(m.Status) == "sedang" {
					ongoingMaterials = append(ongoingMaterials, m)
				}
			}
		}
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("📚 **Rekap Belajar Mingguan** — %s", now.Format("02 Jan 2006")), "")

	if len(milestones) > 0 {
		lines = append(lines, "🏆 **Milestone Tercapai Minggu Ini:**")
		for _, m := range milestones {
			lines = append(lines, fmt.Sprintf("• [%s] %s", strings.ToUpper(m.Topik), m.Kegiatan))
		}
		lines = append(lines, "")
	}

	if len(recentLogs) > 0 {
		lines = append(lines, fmt.Sprintf("✍️ **Sesi Belajar Tercatat (%d sesi):**", len(recentLogs)))
		limit := len(recentLogs)
		if limit > 5 {
			limit = 5
		}
		for _, l := range recentLogs[:limit] {
			lines = append(lines, fmt.Sprintf("• %s %s", l.Tanggal[5:], l.Kegiatan))
		}
		if len(recentLogs) > 5 {
			lines = append(lines, fmt.Sprintf("_...dan %d sesi lainnya_", len(recentLogs)-5))
		}
		lines = append(lines, "")
	}

	if len(ongoingMaterials) > 0 {
		lines = append(lines, "📖 **Materi yang Sedang Dipelajari:**")
		for _, mat := range ongoingMaterials {
			lines = append(lines, fmt.Sprintf("• %s (%s)", mat.Judul, mat.Topik))
		}
		lines = append(lines, "")
	}

	if len(milestones) == 0 && len(recentLogs) == 0 && len(ongoingMaterials) == 0 {
		lines = append(lines,
			"Belum ada aktivitas belajar yang tercatat minggu ini.",
			"💡 _Bagikan buku, artikel, atau kursus yang ingin kamu pelajari di channel learning materials._",
		)
	} else {
		lines = append(lines, "🌟 _Konsistensi kecil setiap hari membawa hasil besar. Siap lanjut minggu depan?_")
	}

	message := strings.Join(lines, "\n")

	if isDryRun {
		log.Printf("[learning_weekly] dry-run preview (channel: %d):", channelID)
		fmt.Println(message)
		return nil
	}

	discordClient := discord.NewClient(token)
	if err := discordClient.SendChannelMessage(channelID, message); err != nil {
		return fmt.Errorf("send discord: %w", err)
	}

	log.Printf("[learning_weekly] posted weekly learning recap to channel %d", channelID)
	return nil
}
