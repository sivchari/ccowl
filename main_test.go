package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFormatCompactTitle(t *testing.T) {
	block := Block{
		CostUSD: 15.58,
	}

	expected := "🦉 $15.58"
	actual := formatCompactTitle(block)

	if actual != expected {
		t.Errorf("Expected %q, got %q", expected, actual)
	}
}

func TestFormatDetailedInfo(t *testing.T) {
	block := Block{
		StartTime:     "2025-06-30T05:00:00.000Z",
		EndTime:       "2025-06-30T10:00:00.000Z",
		ActualEndTime: "2025-06-30T07:00:00.000Z",
		CostUSD:       15.58,
		BurnRate: BurnRate{
			CostPerHour:     12.88,
			TokensPerMinute: 250.0,
		},
		Projection: Projection{
			RemainingMinutes: 177,
			TotalCost:        53.65,
		},
		TotalTokens: 38144,
		Entries:     359,
	}

	actual := formatDetailedInfo(block)
	
	if len(actual) != 8 {
		t.Errorf("Expected 8 items, got %d", len(actual))
	}

	// Check that we have session, cost, burn rate, tokens, and projection info
	requiredInfoEn := []string{"Session", "Current Cost", "Burn Rate", "Tokens Used", "Projected Cost"}
	requiredInfoJa := []string{"セッション", "現在の費用", "消費ペース", "使用トークン", "予想最終費用"}
	
	// Test both English and Japanese depending on locale
	requiredInfo := requiredInfoEn
	if isJapanese {
		requiredInfo = requiredInfoJa
	}
	
	for _, required := range requiredInfo {
		found := false
		for _, item := range actual {
			if strings.Contains(item, required) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing %s information in detailed info", required)
		}
	}
}

func TestCCUsageResponseParsing(t *testing.T) {
	jsonData := `{
		"blocks": [
			{
				"id": "2025-06-30T05:00:00.000Z",
				"startTime": "2025-06-30T05:00:00.000Z",
				"endTime": "2025-06-30T10:00:00.000Z",
				"actualEndTime": "2025-06-30T07:02:38.654Z",
				"isActive": true,
				"isGap": false,
				"entries": 359,
				"tokenCounts": {
					"inputTokens": 1180,
					"outputTokens": 36964,
					"cacheCreationInputTokens": 1062850,
					"cacheReadInputTokens": 18824259
				},
				"totalTokens": 38144,
				"costUSD": 15.581772000000006,
				"models": ["claude-opus-4-20250514", "claude-sonnet-4-20250514"],
				"burnRate": {
					"tokensPerMinute": 525.5185231364406,
					"costPerHour": 12.880416017127851
				},
				"projection": {
					"totalTokens": 131345,
					"totalCost": 53.65,
					"remainingMinutes": 177
				}
			}
		]
	}`

	var response CCUsageResponse
	err := json.Unmarshal([]byte(jsonData), &response)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if len(response.Blocks) != 1 {
		t.Errorf("Expected 1 block, got %d", len(response.Blocks))
	}

	block := response.Blocks[0]
	if !block.IsActive {
		t.Error("Expected block to be active")
	}

	if block.CostUSD != 15.581772000000006 {
		t.Errorf("Expected cost 15.581772000000006, got %f", block.CostUSD)
	}

	if block.Projection.RemainingMinutes != 177 {
		t.Errorf("Expected 177 remaining minutes, got %d", block.Projection.RemainingMinutes)
	}
}