package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/getlantern/systray"
)

type TokenCounts struct {
	InputTokens              int `json:"inputTokens"`
	OutputTokens             int `json:"outputTokens"`
	CacheCreationInputTokens int `json:"cacheCreationInputTokens"`
	CacheReadInputTokens     int `json:"cacheReadInputTokens"`
}

type BurnRate struct {
	TokensPerMinute float64 `json:"tokensPerMinute"`
	CostPerHour     float64 `json:"costPerHour"`
}

type Projection struct {
	TotalTokens      int     `json:"totalTokens"`
	TotalCost        float64 `json:"totalCost"`
	RemainingMinutes int     `json:"remainingMinutes"`
}

type Block struct {
	ID            string      `json:"id"`
	StartTime     string      `json:"startTime"`
	EndTime       string      `json:"endTime"`
	ActualEndTime string      `json:"actualEndTime"`
	IsActive      bool        `json:"isActive"`
	IsGap         bool        `json:"isGap"`
	Entries       int         `json:"entries"`
	TokenCounts   TokenCounts `json:"tokenCounts"`
	TotalTokens   int         `json:"totalTokens"`
	CostUSD       float64     `json:"costUSD"`
	Models        []string    `json:"models"`
	BurnRate      BurnRate    `json:"burnRate"`
	Projection    Projection  `json:"projection"`
}

type CCUsageResponse struct {
	Blocks []Block `json:"blocks"`
}

var isJapanese bool

func init() {
	// Detect system locale
	lang := os.Getenv("LANG")
	if lang == "" {
		// Fallback to checking other environment variables
		lang = os.Getenv("LC_ALL")
		if lang == "" {
			lang = os.Getenv("LC_MESSAGES")
		}
	}

	// Check if Japanese locale is set
	isJapanese = strings.HasPrefix(strings.ToLower(lang), "ja")
}

func t(en, ja string) string {
	if isJapanese {
		return ja
	}
	return en
}

func getCCUsageData() (*CCUsageResponse, error) {
	cmd := exec.Command("ccusage", "blocks", "--json", "--active")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute ccusage command: %w", err)
	}

	var response CCUsageResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return &response, nil
}

func formatCompactTitle(block Block) string {
	return fmt.Sprintf("🦉 $%.2f", block.CostUSD)
}

func formatNumber(num int) string {
	switch {
	case num >= 1000000:
		return fmt.Sprintf("%.1fM", float64(num)/1000000)
	case num >= 1000:
		return fmt.Sprintf("%.1fk", float64(num)/1000)
	default:
		return fmt.Sprintf("%d", num)
	}
}

func formatBurnRate(tokensPerMin float64) string {
	switch {
	case tokensPerMin >= 1000000:
		return fmt.Sprintf("%.1fM", tokensPerMin/1000000)
	case tokensPerMin >= 1000:
		return fmt.Sprintf("%.1fk", tokensPerMin/1000)
	default:
		return fmt.Sprintf("%.0f", tokensPerMin)
	}
}

func getBurnRateStatus(tokensPerMin float64) string {
	switch {
	case tokensPerMin < 300:
		return "🟢 LOW"
	case tokensPerMin < 700:
		return "🟡 MODERATE"
	default:
		return "🔴 HIGH"
	}
}

func getSessionProgress(startTime, endTime, actualEndTime string) (string, string) {
	start, _ := time.Parse(time.RFC3339, startTime)
	end, _ := time.Parse(time.RFC3339, endTime)
	actual, _ := time.Parse(time.RFC3339, actualEndTime)

	remaining := end.Sub(actual)
	remainingHours := int(remaining.Hours())
	remainingMins := int(remaining.Minutes()) % 60

	remainingStr := fmt.Sprintf("%dm", remainingMins)
	if remainingHours > 0 {
		remainingStr = fmt.Sprintf("%dh %dm", remainingHours, remainingMins)
	}

	return remainingStr, start.Format("15:04")
}

func formatWithCommas(n int) string {
	s := fmt.Sprintf("%d", n)
	if n < 0 {
		s = s[1:]
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := strings.Join(parts, ",")
	if n < 0 {
		out = "-" + out
	}
	return out
}

func modelsList(models []string) string {
	if len(models) == 0 {
		return t("n/a", "なし")
	}
	return strings.Join(models, ", ")
}

// formatDetailedInfo returns parallel slices of menu titles and their tooltips.
func formatDetailedInfo(block Block) (titles []string, tooltips []string) {
	remaining, startTime := getSessionProgress(block.StartTime, block.EndTime, block.ActualEndTime)
	burnRateStatus := getBurnRateStatus(block.BurnRate.TokensPerMinute)
	endTime := ""
	if end, err := time.Parse(time.RFC3339, block.EndTime); err == nil {
		endTime = end.Format("15:04")
	}
	tc := block.TokenCounts

	titles = []string{
		fmt.Sprintf("⏱️ %s: %s %s / %s %s",
			t("Session", "セッション"),
			t("Started", "開始"), startTime,
			t("Remaining", "残り"), remaining),
		"",
		fmt.Sprintf("💰 %s: $%.2f", t("Current Cost", "現在の費用"), block.CostUSD),
		fmt.Sprintf("🔥 %s: %s (%s %s)", t("Burn Rate", "消費ペース"), burnRateStatus, formatBurnRate(block.BurnRate.TokensPerMinute), t("tokens/min", "トークン/分")),
		fmt.Sprintf("📊 %s: %s", t("Tokens Used", "使用トークン"), formatNumber(block.TotalTokens)),
		"",
		fmt.Sprintf("📈 %s: $%.2f", t("Projected Cost", "予想最終費用"), block.Projection.TotalCost),
		fmt.Sprintf("🎯 %s: %s%s", t("API Calls", "API呼び出し"), formatNumber(block.Entries), t("", "回")),
	}

	tooltips = []string{
		fmt.Sprintf("%s–%s · %s: %s", startTime, endTime, t("Models", "モデル"), modelsList(block.Models)),
		"",
		fmt.Sprintf("$%.2f/%s", block.BurnRate.CostPerHour, t("hour", "時間")),
		fmt.Sprintf("%s %s · $%.2f/%s", formatWithCommas(int(block.BurnRate.TokensPerMinute)), t("tokens/min", "トークン/分"), block.BurnRate.CostPerHour, t("hour", "時間")),
		fmt.Sprintf("%s: %s · %s: %s · %s: %s · %s: %s",
			t("Input", "入力"), formatNumber(tc.InputTokens),
			t("Output", "出力"), formatNumber(tc.OutputTokens),
			t("Cache write", "キャッシュ書込"), formatNumber(tc.CacheCreationInputTokens),
			t("Cache read", "キャッシュ読込"), formatNumber(tc.CacheReadInputTokens)),
		"",
		fmt.Sprintf("%s %s · %s %d %s", t("Projected", "予想"), formatNumber(block.Projection.TotalTokens), t("tokens,", "トークン、"), block.Projection.RemainingMinutes, t("min left", "分残り")),
		fmt.Sprintf("%s: %s", t("Models", "モデル"), modelsList(block.Models)),
	}
	return titles, tooltips
}

func main() {
	systray.Run(onReady, onExit)
}

var (
	menuItems []*systray.MenuItem
)

func onReady() {
	systray.SetTitle(fmt.Sprintf("🦉 %s...", t("Loading", "読み込み中")))
	systray.SetTooltip("Claude Cost Monitor")

	// Create placeholder menu items (will be dynamically updated)
	for range 10 {
		menuItems = append(menuItems, systray.AddMenuItem(t("Loading...", "読み込み中..."), "Loading..."))
	}

	systray.AddSeparator()
	mQuit := systray.AddMenuItem(t("Quit", "終了"), "Quit the application")

	// Start the update loop
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		// Initial update
		updateStatus()

		for {
			select {
			case <-ticker.C:
				updateStatus()
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func updateStatus() {
	data, err := getCCUsageData()
	if err != nil {
		log.Printf("Error getting usage data: %v", err)
		systray.SetTitle(fmt.Sprintf("🦉 %s", t("Error", "エラー")))
		updateMenuItems([]string{fmt.Sprintf("❌ %s", t("Failed to fetch data", "データを取得できませんでした"))}, nil)
		return
	}

	if len(data.Blocks) == 0 {
		systray.SetTitle(fmt.Sprintf("🦉 %s", t("No Data", "データなし")))
		updateMenuItems([]string{fmt.Sprintf("⚠️ %s", t("No data available", "データがありません"))}, nil)
		return
	}

	// Find the active block
	activeBlock := findActiveBlock(data.Blocks)

	if activeBlock == nil {
		systray.SetTitle(fmt.Sprintf("🦉 %s", t("Inactive", "非アクティブ")))
		updateMenuItems([]string{fmt.Sprintf("💤 %s", t("No active session", "アクティブなセッションがありません"))}, nil)
		return
	}

	// Update compact title
	compactTitle := formatCompactTitle(*activeBlock)
	systray.SetTitle(compactTitle)

	// Update detailed menu items
	titles, tooltips := formatDetailedInfo(*activeBlock)
	updateMenuItems(titles, tooltips)
}

func updateMenuItems(titles []string, tooltips []string) {
	for i, item := range menuItems {
		if i < len(titles) {
			if titles[i] == "" {
				item.Hide()
			} else {
				item.Show()
				item.SetTitle(titles[i])
				tip := titles[i]
				if i < len(tooltips) && tooltips[i] != "" {
					tip = tooltips[i]
				}
				item.SetTooltip(tip)
			}
		} else {
			item.Hide()
		}
	}
}

func onExit() {}

func findActiveBlock(blocks []Block) *Block {
	for _, block := range blocks {
		if block.IsActive {
			return &block
		}
	}
	return nil
}
