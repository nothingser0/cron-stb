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

type TasksDailyJob struct{}

func init() {
	Register(&TasksDailyJob{})
}

func (j *TasksDailyJob) Name() string {
	return "tasks_daily"
}

func (j *TasksDailyJob) Description() string {
	return "Morning task & next action focus briefing to Discord"
}

func (j *TasksDailyJob) Schedule() string {
	return config.GetString("TASK_SCHEDULE", "0 8 * * *")
}

type taskEntry struct {
	ID         string `json:"id"`
	Task       string `json:"task"`
	Title      string `json:"title"`
	NextAction string `json:"next_action"`
	Action     string `json:"action"`
	Proyek     string `json:"proyek"`
	Project    string `json:"project"`
	Prioritas  string `json:"prioritas"`
	Priority   string `json:"priority"`
	Due        string `json:"due"`
	Deadline   string `json:"deadline"`
	Status     string `json:"status"`
	Blocker    string `json:"blocker"`
}

func (t *taskEntry) GetTask() string {
	if t.Task != "" {
		return t.Task
	}
	return t.Title
}

func (t *taskEntry) GetNextAction() string {
	if t.NextAction != "" {
		return t.NextAction
	}
	return t.Action
}

func (t *taskEntry) GetPrioritas() string {
	if t.Prioritas != "" {
		return strings.ToLower(t.Prioritas)
	}
	if t.Priority != "" {
		return strings.ToLower(t.Priority)
	}
	return "sedang"
}

func (t *taskEntry) GetDue() string {
	if t.Due != "" {
		return t.Due
	}
	return t.Deadline
}

func (j *TasksDailyJob) Run(ctx context.Context) error {
	cfg := config.LoadConfig()
	isDryRun, _ := ctx.Value(DryRunKey).(bool)

	token := cfg.GetJobDiscordToken("TASK_DISCORD_TOKEN")
	if token == "" && !isDryRun {
		return fmt.Errorf("missing DISCORD_BOT_TOKEN")
	}

	channelID := config.GetInt64("TASK_CHANNEL_ID", 1511383869319024730)
	if channelID == 0 && !isDryRun {
		return fmt.Errorf("TASK_CHANNEL_ID not configured")
	}

	now := getWIBNow()
	todayStr := now.Format("2006-01-02")

	files := findDataGlob("tasks", "task-*.jsonl")
	var activeTasks []taskEntry

	for _, file := range files {
		lines, err := readLines(file)
		if err != nil {
			continue
		}
		for _, line := range lines {
			var t taskEntry
			if err := json.Unmarshal([]byte(line), &t); err == nil {
				st := strings.ToLower(t.Status)
				if (st == "todo" || st == "doing" || st == "blocked" || st == "") && t.GetTask() != "" {
					activeTasks = append(activeTasks, t)
				}
			}
		}
	}

	var dueToday, highPriority, others []taskEntry
	for _, t := range activeTasks {
		if t.GetDue() == todayStr {
			dueToday = append(dueToday, t)
		} else if t.GetPrioritas() == "tinggi" || t.GetPrioritas() == "high" {
			highPriority = append(highPriority, t)
		} else {
			others = append(others, t)
		}
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("📋 **Morning Task Briefing** — %s", now.Format("02 Jan 2006")), "")

	if len(activeTasks) == 0 {
		lines = append(lines,
			"Tidak ada task aktif yang menggantung. Luar biasa! 🎉",
			"💡 _Tulis task baru di channel ini untuk otomatis dipecah jadi Next Action fisik._",
		)
	} else {
		if len(dueToday) > 0 {
			lines = append(lines, "🔥 **Jatuh Tempo Hari Ini:**")
			for _, t := range dueToday {
				actionStr := ""
				if action := t.GetNextAction(); action != "" {
					actionStr = fmt.Sprintf("\n   ↳ *Next Action:* %s", action)
				}
				lines = append(lines, fmt.Sprintf("• **%s**%s", t.GetTask(), actionStr))
			}
			lines = append(lines, "")
		}

		lines = append(lines, "🎯 **Rekomendasi Fokus Hari Ini:**")
		fokusList := append(dueToday, highPriority...)
		if len(fokusList) < 3 {
			for _, o := range others {
				fokusList = append(fokusList, o)
				if len(fokusList) >= 3 {
					break
				}
			}
		}

		limit := min(len(fokusList), 3)
		for i, t := range fokusList[:limit] {
			prio := strings.ToUpper(t.GetPrioritas())
			action := t.GetNextAction()
			if action == "" {
				action = "Tentukan langkah fisik pertama"
			}
			lines = append(lines, fmt.Sprintf("%d. `[%s]` **%s**\n   ↳ *Next:* %s", i+1, prio, t.GetTask(), action))
		}
		lines = append(lines, "")

		lines = append(lines, fmt.Sprintf("📊 _Total %d task aktif di antrean._", len(activeTasks)))
	}

	message := strings.Join(lines, "\n")

	if isDryRun {
		log.Printf("[tasks_daily] dry-run preview (channel: %d):", channelID)
		fmt.Println(message)
		return nil
	}

	discordClient := discord.NewClient(token)
	if err := discordClient.SendChannelMessage(channelID, message); err != nil {
		return fmt.Errorf("send discord: %w", err)
	}

	log.Printf("[tasks_daily] posted daily tasks to channel %d", channelID)
	return nil
}
