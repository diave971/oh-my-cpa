package quota_test

import (
	"testing"
	"time"

	"github.com/oh-my-cpa/oh-my-cpa/internal/quota"
)

func TestParseCodebuddyUsage(t *testing.T) {
	raw := []byte(`{
		"code": 0,
		"msg": "OK",
		"data": {
			"Response": {
				"Data": {
					"TotalCount": 3,
					"TotalDosage": 3000,
					"Accounts": [
						{
							"PackageName": "CodeBuddy个人体验版",
							"CycleCapacitySize": 500,
							"CycleCapacityUsed": 116,
							"CycleCapacityRemain": 384,
							"CycleStartTime": "2026-09-01 00:00:00",
							"CycleEndTime": "2026-09-30 23:59:59"
						},
						{
							"PackageName": "CodeBuddy个人版国内运营裂变包",
							"CycleCapacitySize": 1500,
							"CycleCapacityUsed": 0,
							"CycleCapacityRemain": 1500,
							"CycleStartTime": "2026-09-14 23:23:39",
							"CycleEndTime": "2026-10-14 23:23:38"
						}
					]
				}
			}
		}
	}`)

	nowMS := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC).UnixMilli()
	windows, plan, err := quota.ParseCodebuddyUsage(raw, nowMS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan == nil || plan.PlanType != "codebuddy" {
		t.Fatalf("expected codebuddy plan, got %v", plan)
	}

	if len(windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(windows))
	}

	w1 := windows[0]
	if w1.Label != "CodeBuddy个人体验版" {
		t.Errorf("expected CodeBuddy个人体验版, got %s", w1.Label)
	}
	if w1.Limit == nil || *w1.Limit != 500 {
		t.Errorf("expected limit 500, got %v", w1.Limit)
	}
	if w1.Used == nil || *w1.Used != 116 {
		t.Errorf("expected used 116, got %v", w1.Used)
	}
	if w1.UsedPercent == nil || *w1.UsedPercent < 23.0 || *w1.UsedPercent > 24.0 {
		t.Errorf("expected ~23.2%% used, got %v", w1.UsedPercent)
	}
}
