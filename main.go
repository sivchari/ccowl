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
	cmd := exec.Command("ccusage", "blocks", "--live", "--json", "--active")
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

func formatDetailedInfo(block Block) []string {
	remaining, startTime := getSessionProgress(block.StartTime, block.EndTime, block.ActualEndTime)
	burnRateStatus := getBurnRateStatus(block.BurnRate.TokensPerMinute)

	return []string{
		fmt.Sprintf("⏱️ %s: %s %s / %s %s",
			t("Session", "セッション"),
			t("Started", "開始"), startTime,
			t("Remaining", "残り"), remaining),
		"",
		fmt.Sprintf("💰 %s: $%.2f", t("Current Cost", "現在の費用"), block.CostUSD),
		fmt.Sprintf("🔥 %s: %s (%.0f token/min)", t("Burn Rate", "消費ペース"), burnRateStatus, block.BurnRate.TokensPerMinute),
		fmt.Sprintf("📊 %s: %s", t("Tokens Used", "使用トークン"), formatNumber(block.TotalTokens)),
		"",
		fmt.Sprintf("📈 %s: $%.2f", t("Projected Cost", "予想最終費用"), block.Projection.TotalCost),
		fmt.Sprintf("🎯 %s: %s%s", t("API Calls", "API呼び出し"), formatNumber(block.Entries), t("", "回")),
	}
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
		updateMenuItems([]string{fmt.Sprintf("❌ %s", t("Failed to fetch data", "データを取得できませんでした"))})
		return
	}

	if len(data.Blocks) == 0 {
		systray.SetTitle(fmt.Sprintf("🦉 %s", t("No Data", "データなし")))
		updateMenuItems([]string{fmt.Sprintf("⚠️ %s", t("No data available", "データがありません"))})
		return
	}

	// Find the active block
	activeBlock := findActiveBlock(data.Blocks)

	if activeBlock == nil {
		systray.SetTitle(fmt.Sprintf("🦉 %s", t("Inactive", "非アクティブ")))
		updateMenuItems([]string{fmt.Sprintf("💤 %s", t("No active session", "アクティブなセッションがありません"))})
		return
	}

	// Update compact title
	compactTitle := formatCompactTitle(*activeBlock)
	systray.SetTitle(compactTitle)

	// Update detailed menu items
	detailedInfo := formatDetailedInfo(*activeBlock)
	updateMenuItems(detailedInfo)
}

func updateMenuItems(info []string) {
	for i, item := range menuItems {
		if i < len(info) {
			if info[i] == "" {
				item.Hide()
			} else {
				item.Show()
				item.SetTitle(info[i])
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
