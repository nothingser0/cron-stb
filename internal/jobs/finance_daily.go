package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"cron-stb/internal/config"
	"cron-stb/internal/discord"
)

type FinanceDailyJob struct{}

func init() {
	Register(&FinanceDailyJob{})
}

func (j *FinanceDailyJob) Name() string {
	return "finance_daily"
}

func (j *FinanceDailyJob) Description() string {
	return "Daily finance summary and expense recap (only sends when transactions exist)"
}

func (j *FinanceDailyJob) Schedule() string {
	return config.GetString("FINANCE_SCHEDULE", "0 22 * * *")
}

type financeEntry struct {
	ID          string  `json:"id"`
	Tanggal     string  `json:"tanggal"`
	Date        string  `json:"date"`
	Tipe        string  `json:"tipe"`
	Type        string  `json:"type"`
	Nominal     float64 `json:"nominal"`
	Amount      float64 `json:"amount"`
	Kategori    string  `json:"kategori"`
	Category    string  `json:"category"`
	Deskripsi   string  `json:"deskripsi"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
}

func (e *financeEntry) GetNominal() float64 {
	if e.Nominal > 0 {
		return e.Nominal
	}
	return e.Amount
}

func (e *financeEntry) GetTipe() string {
	if e.Tipe != "" {
		return strings.ToLower(e.Tipe)
	}
	return strings.ToLower(e.Type)
}

func (e *financeEntry) GetKategori() string {
	if e.Kategori != "" {
		return strings.ToLower(e.Kategori)
	}
	if e.Category != "" {
		return strings.ToLower(e.Category)
	}
	return "lainnya"
}

func (e *financeEntry) GetDeskripsi() string {
	if e.Deskripsi != "" {
		return e.Deskripsi
	}
	return e.Description
}

func (j *FinanceDailyJob) Run(ctx context.Context) error {
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
	todayStr := now.Format("2006-01-02")
	filename := fmt.Sprintf("transaksi-%s.jsonl", todayStr)

	filePath := findDataFile("finance", filename)
	var entries []financeEntry

	if filePath != "" {
		lines, err := readLines(filePath)
		if err == nil {
			for _, line := range lines {
				var e financeEntry
				if err := json.Unmarshal([]byte(line), &e); err == nil {
					if e.Status != "dihapus" && e.GetNominal() > 0 {
						entries = append(entries, e)
					}
				}
			}
		}
	}

	// Harian bersyarat: Jika tidak ada transaksi hari ini, jangan kirim pesan ke Discord
	if len(entries) == 0 {
		log.Printf("[finance_daily] no transactions recorded for today (%s), skipping Discord message", todayStr)
		return nil
	}

	var totalIncome, totalExpense float64
	catMap := make(map[string]float64)

	for _, e := range entries {
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

	var lines []string
	lines = append(lines, fmt.Sprintf("📊 **Rekap Keuangan Harian** — %s", now.Format("02 Jan 2006")), "")
	lines = append(lines,
		fmt.Sprintf("💰 **Pemasukan:** Rp %s", formatRupiah(totalIncome)),
		fmt.Sprintf("💸 **Pengeluaran:** Rp %s", formatRupiah(totalExpense)),
		fmt.Sprintf("⚖️ **Selisih Bersih:** %sRp %s", formatNetSign(net), formatRupiah(absFloat(net))),
		"",
	)

	if len(catMap) > 0 {
		lines = append(lines, "📂 **Rincian Pengeluaran:**")
		type catSum struct {
			name  string
			total float64
		}
		var list []catSum
		for k, v := range catMap {
			list = append(list, catSum{name: k, total: v})
		}
		sort.Slice(list, func(i, j int) bool {
			return list[i].total > list[j].total
		})
		for _, item := range list {
			lines = append(lines, fmt.Sprintf("• %s: Rp %s", strings.Title(item.name), formatRupiah(item.total)))
		}
		lines = append(lines, "")
	}

	lines = append(lines, fmt.Sprintf("📝 _Total %d transaksi tercatat hari ini._", len(entries)))
	message := strings.Join(lines, "\n")

	if isDryRun {
		log.Printf("[finance_daily] dry-run preview (channel: %d):", channelID)
		fmt.Println(message)
		return nil
	}

	discordClient := discord.NewClient(token)
	if err := discordClient.SendChannelMessage(channelID, message); err != nil {
		return fmt.Errorf("send discord: %w", err)
	}

	log.Printf("[finance_daily] posted daily summary to channel %d", channelID)
	return nil
}
