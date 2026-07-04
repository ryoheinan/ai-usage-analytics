package store

import (
	"context"
	"testing"
	"time"
)

func TestStoreSummaryAndHealth(t *testing.T) {
	db, err := Open(t.TempDir() + "/test.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	success := true
	duration := int64(125)
	err = db.InsertEvents(context.Background(), []Event{{
		Timestamp:            time.Now().UTC(),
		Model:                "gpt-test",
		Name:                 "codex.api_request",
		Success:              &success,
		DurationMS:           &duration,
		InputTokens:          100,
		CachedInputTokens:    20,
		CacheCreationTokens:  10,
		OutputTokens:         50,
		TotalTokens:          180,
		EstimatedCostUSD:     0.001,
		DroppedContentFields: 2,
	}})
	if err != nil {
		t.Fatal(err)
	}

	summary, err := db.Summary(context.Background(), time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if summary.Events != 1 || summary.Requests != 1 || summary.TotalTokens != 180 || summary.CacheCreationTokens != 10 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	health, err := db.IngestionHealth(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if health.AcceptedEvents != 1 || health.DroppedContentFields != 2 || health.LastEventAt == nil {
		t.Fatalf("unexpected health: %+v", health)
	}

	sources, err := db.SourceBreakdown(context.Background(), time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].Source != "codex" || sources[0].Requests != 1 || sources[0].TotalTokens != 180 {
		t.Fatalf("unexpected source breakdown: %+v", sources)
	}
}

func TestStoreSeriesFillsEmptyDays(t *testing.T) {
	db, err := Open(t.TempDir() + "/test.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Now().UTC()
	success := true
	err = db.InsertEvents(context.Background(), []Event{{
		Timestamp:    now,
		Model:        "gpt-test",
		Name:         "codex.api_request",
		Success:      &success,
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
	}})
	if err != nil {
		t.Fatal(err)
	}

	series, err := db.Series(context.Background(), now.AddDate(0, 0, -6))
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 7 {
		t.Fatalf("len(Series()) = %d, want 7: %#v", len(series), series)
	}
	if series[0].Bucket != now.AddDate(0, 0, -6).Format("2006-01-02") {
		t.Fatalf("first bucket = %q, want six days ago", series[0].Bucket)
	}
	last := series[len(series)-1]
	if last.Bucket != now.Format("2006-01-02") || last.TotalTokens != 150 || last.Requests != 1 {
		t.Fatalf("last bucket = %+v, want today's totals", last)
	}
	for _, point := range series[:len(series)-1] {
		if point.TotalTokens != 0 || point.Events != 0 {
			t.Fatalf("empty day has data: %+v", point)
		}
	}
}

func TestStoreReturnsEmptySlices(t *testing.T) {
	db, err := Open(t.TempDir() + "/test.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	series, err := db.Series(context.Background(), time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if series == nil || len(series) != 1 || series[0].TotalTokens != 0 {
		t.Fatalf("Series() = %#v, want one zero-filled point", series)
	}

	models, err := db.ModelBreakdown(context.Background(), time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if models == nil || len(models) != 0 {
		t.Fatalf("ModelBreakdown() = %#v, want empty non-nil slice", models)
	}

	sources, err := db.SourceBreakdown(context.Background(), time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if sources == nil || len(sources) != 0 {
		t.Fatalf("SourceBreakdown() = %#v, want empty non-nil slice", sources)
	}
}

func TestStoreSeparatesSourceAndModelBreakdowns(t *testing.T) {
	db, err := Open(t.TempDir() + "/test.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Now().UTC()
	success := true
	err = db.InsertEvents(context.Background(), []Event{
		{
			Timestamp:        now,
			Source:           "codex",
			Model:            "shared-model",
			Name:             "codex.api_request",
			Success:          &success,
			TotalTokens:      100,
			EstimatedCostUSD: 0.001,
		},
		{
			Timestamp:        now,
			Source:           "claude-code",
			Model:            "shared-model",
			Name:             "claude_code.api_request",
			Success:          &success,
			TotalTokens:      200,
			EstimatedCostUSD: 0.002,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	models, err := db.ModelBreakdown(context.Background(), now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Fatalf("ModelBreakdown() rows = %d, want 2: %+v", len(models), models)
	}
	if models[0].Source != "claude-code" || models[0].TotalTokens != 200 {
		t.Fatalf("first model row = %+v, want Claude row first", models[0])
	}

	summary, err := db.Summary(context.Background(), now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if summary.Requests != 2 || summary.TotalTokens != 300 {
		t.Fatalf("summary = %+v, want both request names counted", summary)
	}

	claudeSummary, err := db.SummaryBySource(context.Background(), now.Add(-time.Hour), "claude-code")
	if err != nil {
		t.Fatal(err)
	}
	if claudeSummary.Requests != 1 || claudeSummary.TotalTokens != 200 {
		t.Fatalf("claude summary = %+v, want only Claude row", claudeSummary)
	}

	codexModels, err := db.ModelBreakdownBySource(context.Background(), now.Add(-time.Hour), "codex")
	if err != nil {
		t.Fatal(err)
	}
	if len(codexModels) != 1 || codexModels[0].Source != "codex" || codexModels[0].TotalTokens != 100 {
		t.Fatalf("codex model breakdown = %+v, want only Codex row", codexModels)
	}
}
