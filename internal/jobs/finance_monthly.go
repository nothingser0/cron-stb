package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"cron-stb/internal/config"
	"cron-stb/internal/discord"
)

type FinanceMonthlyJob struct{}

func init() {
	Register(&FinanceMonthlyJob{})
}

func (j *FinanceMonthlyJob) Name() string {
	return "finance_monthly"
}

func (j *FinanceMonthlyJob) Description() string {
	return "Monthly finance summary, net cashflow, and spending breakdown to Discord"
}

func (j *FinanceMonthlyJob) Schedule() string {
	return config.GetString("FINANCE_MONTHLY_SCHEDULE", "0 8 1 * *")
}

func (j *FinanceMonthlyJob) Run(ctx context.Context) error {
	cfg := config.LoadConfig()
	isDryRun, _ := ctx.Value(DryRunKey).(bool)

	token := cfg.GetJobDiscordToken("FINANCE_DISCORD_TOKEN")
	if token == "" && !isDryRun {
		return fmt.Errorf("missing DISCORD_BOT_TOKEN")
	}

	channelID := config.GetInt64("FINANCE_CHANNEL_ID", 0)
	if channelID == 0 && !isDryRun {
		return fmt.Errorf("FINANCE_CHANNEL_ID not configured")
	}

	now := getWIBNow()
	// Evaluasi bulan sebelumnya jika run tanggal 1-5, atau bulan ini jika manual di pertengahan bulan
	targetMonth := now.AddDate(0, -1, 0)
	isManual, _ := ctx.Value(ManualRunKey).(bool)
	if isManual && now.Day() > 5 {
		targetMonth = now
	}

	monthPattern := fmt.Sprintf("transaksi-%s-*.jsonl", targetMonth.Format("2006-01"))
	files := findDataGlob("finance", monthPattern)

	var allEntries []financeEntry
	activeDaysMap := make(map[string]bool)

	for _, file := range files {
		lines, err := readLines(file)
		if err != nil {
			continue
		}
		for _, line := range lines {
			var e financeEntry
			if err := json.Unmarshal([]byte(line), &e); err == nil {
				if e.Status != "dihapus" && e.GetNominal() > 0 {
					allEntries = append(allEntries, e)
					tgl := e.Tanggal
					if tgl == "" {
						tgl = e.Date
					}
					if tgl != "" {
						activeDaysMap[tgl] = true
					}
				}
			}
		}
	}

	var lines []string
	monthName := targetMonth.Format("January 2006")
	lines = append(lines, fmt.Sprintf("🗓️ **Rekap Keuangan Bulanan** — %s", monthName), "")

	if len(allEntries) == 0 {
		lines = append(lines, fmt.Sprintf("Tidak ada data transaksi yang tercatat untuk bulan %s.", monthName))
	} else {
		var totalIncome, totalExpense float64
		catMap := make(map[string]float64)

		for _, e := range allEntries {
			nom := e.GetNominal()
			switch e.GetTipe() {
			case "pemasukan", "income":
				totalIncome += nom
			case "pengeluaran", "expense":
				totalExpense += nom
				catMap[e.GetKategori()] += nom
			}
		}

		net := totalIncome - totalExpense
		activeDays := len(activeDaysMap)
		if activeDays == 0 {
			activeDays = 1
		}
		avgDaily := totalExpense / float64(daysInMonth(targetMonth))

		lines = append(lines,
			fmt.Sprintf("💰 **Total Pemasukan:** Rp %s", formatRupiah(totalIncome)),
			fmt.Sprintf("💸 **Total Pengeluaran:** Rp %s", formatRupiah(totalExpense)),
			fmt.Sprintf("⚖️ **Arus Kas Bersih (Net):** %sRp %s", formatNetSign(net), formatRupiah(absFloat(net))),
			fmt.Sprintf("📈 **Rata-rata Pengeluaran/Hari:** Rp %s", formatRupiah(avgDaily)),
			"",
		)

		if len(catMap) > 0 {
			lines = append(lines, "🏆 **Kategori Pengeluaran Terbesar:**")
			type catSum struct {
				name  string
				total float64
				pct   float64
			}
			var list []catSum
			for k, v := range catMap {
				pct := 0.0
				if totalExpense > 0 {
					pct = (v / totalExpense) * 100
				}
				list = append(list, catSum{name: k, total: v, pct: pct})
			}
			sort.Slice(list, func(i, j int) bool {
				return list[i].total > list[j].total
			})

			for i, item := range list {
				lines = append(lines, fmt.Sprintf("%d. **%s**: Rp %s (%.1f%%)", i+1, strings.Title(item.name), formatRupiah(item.total), item.pct))
				if i >= 4 { // Tampilkan top 5
					break
				}
			}
			lines = append(lines, "")
		}

		lines = append(lines, fmt.Sprintf("📊 _Total %d transaksi dari %d hari belanja aktif tercatat._", len(allEntries), len(activeDaysMap)))
	}

	message := strings.Join(lines, "\n")

	if isDryRun {
		log.Printf("[finance_monthly] dry-run preview (channel: %d):", channelID)
		fmt.Println(message)
		return nil
	}

	discordClient := discord.NewClient(token)
	if err := discordClient.SendChannelMessage(channelID, message); err != nil {
		return fmt.Errorf("send discord: %w", err)
	}

	log.Printf("[finance_monthly] posted monthly summary to channel %d", channelID)
	return nil
}

func daysInMonth(t time.Time) int {
	year, month, _ := t.Date()
	return time.Date(year, month+1, 0, 0, 0, 0, 0, t.Location()).Day()
}
