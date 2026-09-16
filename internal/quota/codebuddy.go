package quota

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// RawCodebuddyAccount represents a single quota/package bucket returned by
// Codebuddy's get-user-resource endpoint.
type RawCodebuddyAccount struct {
	PackageName                string `json:"PackageName"`
	CapacityRemain             any    `json:"CapacityRemain"`
	CycleCapacityRemain        any    `json:"CycleCapacityRemain"`
	CycleCapacityRemainPrecise  string `json:"CycleCapacityRemainPrecise"`
	CycleCapacitySize          any    `json:"CycleCapacitySize"`
	CycleCapacitySizePrecise    string `json:"CycleCapacitySizePrecise"`
	CycleCapacityUsed          any    `json:"CycleCapacityUsed"`
	CycleCapacityUsedPrecise    string `json:"CycleCapacityUsedPrecise"`
	CycleStartTime             string `json:"CycleStartTime"`
	CycleEndTime               string `json:"CycleEndTime"`
	ValidPeriodDays            any    `json:"ValidPeriodDays"`
}

// RawCodebuddyResponse represents the nested wrapper in Codebuddy API.
type RawCodebuddyResponse struct {
	Code int `json:"code"`
	Data struct {
		Response struct {
			Data struct {
				TotalCount  int                   `json:"TotalCount"`
				TotalDosage any                   `json:"TotalDosage"`
				Accounts    []RawCodebuddyAccount `json:"Accounts"`
			} `json:"Data"`
		} `json:"Response"`
	} `json:"data"`
}

// ParseCodebuddyUsage converts Codebuddy get-user-resource raw JSON response into
// normalized quota windows.
func ParseCodebuddyUsage(raw []byte, nowMS int64) ([]QuotaWindow, *QuotaPlan, error) {
	var resp RawCodebuddyResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, nil, fmt.Errorf("invalid codebuddy payload: %w", err)
	}

	accounts := resp.Data.Response.Data.Accounts
	if len(accounts) == 0 {
		// Tolerate direct flat format if any
		var flat struct {
			TotalCount  int                   `json:"TotalCount"`
			TotalDosage any                   `json:"TotalDosage"`
			Accounts    []RawCodebuddyAccount `json:"Accounts"`
		}
		if err := json.Unmarshal(raw, &flat); err == nil && len(flat.Accounts) > 0 {
			accounts = flat.Accounts
		}
	}

	windows := make([]QuotaWindow, 0, len(accounts))
	var totalLimit float64
	var totalRemain float64
	var totalUsed float64
	hasAnyLimit := false

	for i, acct := range accounts {
		label := strings.TrimSpace(acct.PackageName)
		if label == "" {
			label = fmt.Sprintf("权益包 %d", i+1)
		}

		limitVal, hasLim := toFloat(acct.CycleCapacitySize)
		if !hasLim {
			limitVal, hasLim = toFloat(acct.CycleCapacitySizePrecise)
		}

		usedVal, hasUsed := toFloat(acct.CycleCapacityUsed)
		if !hasUsed {
			usedVal, hasUsed = toFloat(acct.CycleCapacityUsedPrecise)
		}

		remainVal, hasRem := toFloat(acct.CycleCapacityRemain)
		if !hasRem {
			remainVal, hasRem = toFloat(acct.CycleCapacityRemainPrecise)
		}

		if !hasUsed && hasLim && hasRem {
			usedVal = limitVal - remainVal
			if usedVal < 0 {
				usedVal = 0
			}
			hasUsed = true
		}

		var usedPercent *float64
		var remainingPercent *float64
		var usedPtr *float64
		var limitPtr *float64

		if hasUsed {
			usedPtr = &usedVal
		}
		if hasLim {
			limitPtr = &limitVal
			totalLimit += limitVal
			hasAnyLimit = true
		}
		if hasUsed {
			totalUsed += usedVal
		}
		if hasRem {
			totalRemain += remainVal
		}

		if hasUsed && hasLim && limitVal > 0 {
			usedPct := clamp(usedVal/limitVal*100, 0, 100)
			remainingPct := clamp(100-usedPct, 0, 100)
			usedPercent = &usedPct
			remainingPercent = &remainingPct
		} else if hasUsed && usedVal > 0 && (!hasLim || limitVal == 0) {
			usedPct := 100.0
			remainingPct := 0.0
			usedPercent = &usedPct
			remainingPercent = &remainingPct
		}

		var resetAtMS *int64
		var resetLabel string
		var periodHours *float64

		if acct.CycleEndTime != "" {
			var endT time.Time
			var parseErr error
			if strings.Contains(acct.CycleEndTime, "T") {
				endT, parseErr = time.Parse(time.RFC3339, acct.CycleEndTime)
			} else {
				loc, _ := time.LoadLocation("Asia/Shanghai")
				if loc == nil {
					loc = time.Local
				}
				endT, parseErr = time.ParseInLocation("2006-01-02 15:04:05", acct.CycleEndTime, loc)
			}
			if parseErr == nil {
				ms := endT.UnixMilli()
				resetAtMS = &ms
				resetLabel = formatResetInstant(ms, nowMS)

				if acct.CycleStartTime != "" {
					var startT time.Time
					if strings.Contains(acct.CycleStartTime, "T") {
						startT, _ = time.Parse(time.RFC3339, acct.CycleStartTime)
					} else {
						loc, _ := time.LoadLocation("Asia/Shanghai")
						if loc == nil {
							loc = time.Local
						}
						startT, _ = time.ParseInLocation("2006-01-02 15:04:05", acct.CycleStartTime, loc)
					}
					if !startT.IsZero() && endT.After(startT) {
						hours := endT.Sub(startT).Hours()
						periodHours = &hours
					}
				}
			}
		}

		windows = append(windows, QuotaWindow{
			ID:               fmt.Sprintf("codebuddy_%d", i),
			Label:            label,
			Kind:             "custom",
			Scope:            "account",
			Used:             usedPtr,
			Limit:            limitPtr,
			UsedPercent:      usedPercent,
			RemainingPercent: remainingPercent,
			ResetAtMS:        resetAtMS,
			ResetLabel:       resetLabel,
			PeriodHours:      periodHours,
			ResetAccuracy:    "exact",
		})
	}

	planLabel := "Codebuddy 账户"
	if hasAnyLimit && totalLimit > 0 {
		planLabel = fmt.Sprintf("Codebuddy (总额度: %.0f / 剩余: %.0f credits)", totalLimit, totalRemain)
	} else if len(accounts) > 0 {
		planLabel = fmt.Sprintf("Codebuddy (%s)", accounts[0].PackageName)
	}

	plan := &QuotaPlan{
		PlanType:  "codebuddy",
		PlanLabel: planLabel,
		Tier:      "standard",
	}

	return windows, plan, nil
}
