package jobs

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"cron-stb/internal/config"
	"cron-stb/internal/discord"
	"cron-stb/internal/sheets"
)

type AlertH1Job struct{}

func init() {
	Register(&AlertH1Job{})
}

func (j *AlertH1Job) Name() string {
	return "alert_airdrop_h1"
}

func (j *AlertH1Job) Description() string {
	return "Alert H-1 airdrop deadline from Google Sheets H-3 Garap"
}

func (j *AlertH1Job) Schedule() string {
	return config.GetString("ALERT_H1_SCHEDULE", "0 21 * * *")
}

type AirdropItem struct {
	Name         string
	Reward       string
	RewardNum    float64
	TanggalGarap time.Time
}

func (j *AlertH1Job) Run(ctx context.Context) error {
	cfg := config.LoadConfig()
	isDryRun, _ := ctx.Value(DryRunKey).(bool)

	token := cfg.GetJobDiscordToken("ALERT_H1_DISCORD_TOKEN")
	if token == "" && !isDryRun {
		return fmt.Errorf("missing DISCORD_BOT_TOKEN")
	}

	if cfg.TokenFile == "" {
		return fmt.Errorf("token.json not found")
	}

	sheetID := cfg.GetJobSheetID("ALERT_H1_SHEET_ID")
	if sheetID == "" {
		return fmt.Errorf("SHEET_ID not configured")
	}

	channelID := config.GetInt64("ALERT_H1_CHANNEL_ID", 0)
	if channelID == 0 && !isDryRun {
		return fmt.Errorf("ALERT_H1_CHANNEL_ID not configured")
	}

	sheetName := config.GetString("ALERT_H1_SHEET_NAME", "H-3 Garap")

	client, err := sheets.NewClient(ctx, cfg.TokenFile, cfg.CredentialsFile)
	if err != nil {
		return fmt.Errorf("sheets client: %w", err)
	}

	rows, err := client.ReadSheet(sheetID, sheetName)
	if err != nil {
		return fmt.Errorf("read %s: %w", sheetName, err)
	}

	items := filterH1Items(rows)
	if len(items) == 0 {
		log.Println("[alert_airdrop_h1] no H-1 items found")
		return nil
	}

	message := formatAlertMessage(items)
	if message == "" {
		return nil
	}

	if isDryRun {
		log.Printf("[alert_airdrop_h1] dry-run preview (%d items, channel: %d):", len(items), channelID)
		fmt.Println(message)
		return nil
	}

	discordClient := discord.NewClient(token)
	if err := discordClient.SendChannelMessage(channelID, message); err != nil {
		return fmt.Errorf("send discord: %w", err)
	}

	log.Printf("[alert_airdrop_h1] posted %d alerts to channel %d", len(items), channelID)
	return nil
}

var reNumbers = regexp.MustCompile(`\d+\.?\d*`)

func parseRewardNumber(rewardStr string) float64 {
	if rewardStr == "" || strings.EqualFold(rewardStr, "N/A") {
		return 0
	}
	matches := reNumbers.FindString(rewardStr)
	if matches != "" {
		val, _ := strconv.ParseFloat(matches, 64)
		return val
	}
	return 0
}

var bulanIndo = map[string]time.Month{
	"januari":   time.January,
	"februari":  time.February,
	"maret":     time.March,
	"april":     time.April,
	"mei":       time.May,
	"juni":      time.June,
	"juli":      time.July,
	"agustus":   time.August,
	"september": time.September,
	"oktober":   time.October,
	"november":  time.November,
	"desember":  time.December,
}

func parseTanggal(raw string, loc *time.Location) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}

	if strings.Contains(raw, "/") {
		if t, err := time.ParseInLocation("02/01/2006", raw, loc); err == nil {
			return t, true
		}
	}

	if strings.Contains(raw, "-") {
		if t, err := time.ParseInLocation("2006-01-02", raw, loc); err == nil {
			return t, true
		}
	}

	if strings.Contains(raw, ",") {
		parts := strings.SplitN(raw, ",", 2)
		if len(parts) == 2 {
			dateParts := strings.Fields(strings.TrimSpace(parts[1]))
			if len(dateParts) == 3 {
				day, err1 := strconv.Atoi(dateParts[0])
				monthName := strings.ToLower(dateParts[1])
				year, err2 := strconv.Atoi(dateParts[2])
				if err1 == nil && err2 == nil {
					if m, ok := bulanIndo[monthName]; ok {
						return time.Date(year, m, day, 0, 0, 0, 0, loc), true
					}
				}
			}
		}
	}

	return time.Time{}, false
}

func filterH1Items(dataRows [][]string) []AirdropItem {
	if len(dataRows) < 6 {
		return nil
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}

	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	tomorrow := today.AddDate(0, 0, 1)

	var filtered []AirdropItem

	for i := 5; i < len(dataRows); i++ {
		row := dataRows[i]
		if len(row) <= 9 {
			continue
		}

		tglDate, ok := parseTanggal(row[9], loc)
		if !ok {
			continue
		}

		if (tglDate.Equal(today) || tglDate.After(today)) && (tglDate.Equal(tomorrow) || tglDate.Before(tomorrow)) {
			name := "N/A"
			if len(row) > 31 && strings.TrimSpace(row[31]) != "" {
				name = strings.TrimSpace(row[31])
			}
			reward := "N/A"
			if len(row) > 14 && strings.TrimSpace(row[14]) != "" {
				reward = strings.TrimSpace(row[14])
			}

			filtered = append(filtered, AirdropItem{
				Name:         name,
				Reward:       reward,
				RewardNum:    parseRewardNumber(reward),
				TanggalGarap: tglDate,
			})
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].RewardNum > filtered[j].RewardNum
	})

	return filtered
}

func formatAlertMessage(items []AirdropItem) string {
	if len(items) == 0 {
		return ""
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	now := time.Now().In(loc)

	totalCount := len(items)
	displayLimit := min(totalCount, 10)
	displayItems := items[:displayLimit]
	remaining := totalCount - displayLimit

	var lines []string
	lines = append(lines,
		fmt.Sprintf("**Airdrop H-1 Alert** — %s", now.Format("02 Jan 2006")),
		"",
		fmt.Sprintf("⚠️ **Deadline ≤ H-1:** %d airdrops", totalCount),
		"",
	)

	if totalCount > 10 {
		lines = append(lines, "**Top 10 by Reward:**")
	} else {
		lines = append(lines, "**List:**")
	}

	for _, item := range displayItems {
		if item.Reward != "" && !strings.EqualFold(item.Reward, "N/A") {
			lines = append(lines, fmt.Sprintf("• %s (%s)", item.Name, item.Reward))
		} else {
			lines = append(lines, fmt.Sprintf("• %s", item.Name))
		}
	}

	if remaining > 0 {
		lines = append(lines, "", fmt.Sprintf("_...and %d more airdrops_", remaining))
	}

	return strings.Join(lines, "\n")
}
