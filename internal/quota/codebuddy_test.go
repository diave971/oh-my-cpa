package quota

import (
	"testing"
	"time"
)

func TestParseCodebuddyUsage(t *testing.T) {
	raw := []byte(`{
		"TotalCount": 3,
		"TotalDosage": 3000,
		"Accounts": [
			{
				"PackageName": "CodeBuddy个人体验版",
				"CapacityRemain": 500,
				"CycleCapacityRemain": 390,
				"CycleCapacityRemainPrecise": "390.5600001",
				"CycleCapacitySize": 500,
				"CycleCapacitySizePrecise": "500",
				"CycleCapacityUsed": 109,
				"CycleCapacityUsedPrecise": "109.4399999",
				"CycleStartTime": "2026-09-01 00:00:00",
				"CycleEndTime": "2026-09-30 23:59:59"
			},
			{
				"PackageName": "CodeBuddy个人版国内运营裂变包",
				"CapacityRemain": 1500,
				"CycleCapacityRemain": 1500,
				"CycleCapacitySize": 1500,
				"CycleCapacityUsed": 0,
				"CycleStartTime": "2026-09-14 23:23:39",
				"CycleEndTime": "2026-10-14 23:23:38"
			}
		]
	}`)

	nowMS := time.Date(2026, 9, 16, 12, 0, 0, 0, time.Local).UnixMilli()
	windows, plan, err := ParseCodebuddyUsage(raw, nowMS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(windows))
	}

	w0 := windows[0]
	if w0.Label != "CodeBuddy个人体验版" {
		t.Errorf("expected label CodeBuddy个人体验版, got %s", w0.Label)
	}
	if w0.Limit == nil || *w0.Limit != 500 {
		t.Errorf("expected limit 500, got %v", w0.Limit)
	}
	if w0.UsedPercent == nil || *w0.UsedPercent < 21.8 || *w0.UsedPercent > 21.9 {
		t.Errorf("expected used percent ~21.88, got %v", w0.UsedPercent)
	}
	if w0.RemainingPercent == nil || *w0.RemainingPercent < 78.1 || *w0.RemainingPercent > 78.2 {
		t.Errorf("expected remaining percent ~78.11, got %v", w0.RemainingPercent)
	}
	if w0.ResetAtMS == nil || *w0.ResetAtMS <= 0 {
		t.Errorf("expected resetAtMS to be populated, got %v", w0.ResetAtMS)
	}

	w1 := windows[1]
	if w1.UsedPercent == nil || *w1.UsedPercent != 0 {
		t.Errorf("expected used percent 0, got %v", w1.UsedPercent)
	}
	if w1.RemainingPercent == nil || *w1.RemainingPercent != 100 {
		t.Errorf("expected remaining percent 100, got %v", w1.RemainingPercent)
	}

	if plan == nil || plan.PlanType != "codebuddy" {
		t.Errorf("expected planType codebuddy, got %v", plan)
	}
}
