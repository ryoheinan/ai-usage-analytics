package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	db *sql.DB
}

type Event struct {
	Timestamp             time.Time
	Source                string
	ConversationID        string
	Model                 string
	Name                  string
	Kind                  string
	Success               *bool
	DurationMS            *int64
	InputTokens           int64
	CachedInputTokens     int64
	CacheCreationTokens   int64
	OutputTokens          int64
	ReasoningOutputTokens int64
	TotalTokens           int64
	EstimatedCostUSD      float64
	DroppedContentFields  int
}

type Summary struct {
	Events                int64   `json:"events"`
	Requests              int64   `json:"requests"`
	Failures              int64   `json:"failures"`
	AvgDurationMS         float64 `json:"avgDurationMs"`
	InputTokens           int64   `json:"inputTokens"`
	CachedInputTokens     int64   `json:"cachedInputTokens"`
	CacheCreationTokens   int64   `json:"cacheCreationTokens"`
	OutputTokens          int64   `json:"outputTokens"`
	ReasoningOutputTokens int64   `json:"reasoningOutputTokens"`
	TotalTokens           int64   `json:"totalTokens"`
	EstimatedCostUSD      float64 `json:"estimatedCostUsd"`
}

type SeriesPoint struct {
	Bucket                string  `json:"bucket"`
	Events                int64   `json:"events"`
	Requests              int64   `json:"requests"`
	Failures              int64   `json:"failures"`
	InputTokens           int64   `json:"inputTokens"`
	CachedInputTokens     int64   `json:"cachedInputTokens"`
	CacheCreationTokens   int64   `json:"cacheCreationTokens"`
	OutputTokens          int64   `json:"outputTokens"`
	ReasoningOutputTokens int64   `json:"reasoningOutputTokens"`
	TotalTokens           int64   `json:"totalTokens"`
	EstimatedCostUSD      float64 `json:"estimatedCostUsd"`
}

type BreakdownRow struct {
	Source           string  `json:"source"`
	Model            string  `json:"model"`
	Events           int64   `json:"events"`
	TotalTokens      int64   `json:"totalTokens"`
	EstimatedCostUSD float64 `json:"estimatedCostUsd"`
}

type SourceBreakdownRow struct {
	Source           string  `json:"source"`
	Events           int64   `json:"events"`
	Requests         int64   `json:"requests"`
	TotalTokens      int64   `json:"totalTokens"`
	EstimatedCostUSD float64 `json:"estimatedCostUsd"`
}

type Health struct {
	LastEventAt          *time.Time `json:"lastEventAt"`
	AcceptedEvents       int64      `json:"acceptedEvents"`
	DroppedContentFields int64      `json:"droppedContentFields"`
}

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	wrapped := &DB{db: db}
	if err := wrapped.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return wrapped, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) migrate(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS telemetry_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ts TEXT NOT NULL,
	source TEXT NOT NULL DEFAULT 'codex',
	conversation_id TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT '',
	success INTEGER,
	duration_ms INTEGER,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	cached_input_tokens INTEGER NOT NULL DEFAULT 0,
	cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	reasoning_output_tokens INTEGER NOT NULL DEFAULT 0,
	total_tokens INTEGER NOT NULL DEFAULT 0,
	estimated_cost_usd REAL NOT NULL DEFAULT 0,
	dropped_content_fields INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_telemetry_events_ts ON telemetry_events(ts);
CREATE INDEX IF NOT EXISTS idx_telemetry_events_source ON telemetry_events(source);
CREATE INDEX IF NOT EXISTS idx_telemetry_events_model ON telemetry_events(model);
CREATE INDEX IF NOT EXISTS idx_telemetry_events_name ON telemetry_events(name);
`)
	if err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	_, _ = d.db.ExecContext(ctx, `ALTER TABLE telemetry_events ADD COLUMN source TEXT NOT NULL DEFAULT 'codex'`)
	_, _ = d.db.ExecContext(ctx, `ALTER TABLE telemetry_events ADD COLUMN cache_creation_tokens INTEGER NOT NULL DEFAULT 0`)
	_, _ = d.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_telemetry_events_source ON telemetry_events(source)`)
	return nil
}

func (d *DB) InsertEvents(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO telemetry_events (
	ts, source, conversation_id, model, name, kind, success, duration_ms,
	input_tokens, cached_input_tokens, cache_creation_tokens, output_tokens, reasoning_output_tokens, total_tokens,
	estimated_cost_usd, dropped_content_fields
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, event := range events {
		var success any
		if event.Success != nil {
			if *event.Success {
				success = 1
			} else {
				success = 0
			}
		}
		var duration any
		if event.DurationMS != nil {
			duration = *event.DurationMS
		}
		_, err := stmt.ExecContext(ctx,
			event.Timestamp.UTC().Format(time.RFC3339Nano),
			defaultSource(event.Source),
			event.ConversationID,
			event.Model,
			event.Name,
			event.Kind,
			success,
			duration,
			event.InputTokens,
			event.CachedInputTokens,
			event.CacheCreationTokens,
			event.OutputTokens,
			event.ReasoningOutputTokens,
			event.TotalTokens,
			event.EstimatedCostUSD,
			event.DroppedContentFields,
		)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) Summary(ctx context.Context, since time.Time) (Summary, error) {
	return d.SummaryBySource(ctx, since, "")
}

func (d *DB) SummaryBySource(ctx context.Context, since time.Time, source string) (Summary, error) {
	var out Summary
	filter, args := sourceFilter(since.UTC().Format(time.RFC3339Nano), source)
	err := d.db.QueryRowContext(ctx, `
SELECT
	COUNT(*),
	COALESCE(SUM(CASE WHEN name IN ('codex.api_request', 'claude_code.api_request') THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END), 0),
	COALESCE(AVG(duration_ms), 0),
	COALESCE(SUM(input_tokens), 0),
	COALESCE(SUM(cached_input_tokens), 0),
	COALESCE(SUM(cache_creation_tokens), 0),
	COALESCE(SUM(output_tokens), 0),
	COALESCE(SUM(reasoning_output_tokens), 0),
	COALESCE(SUM(total_tokens), 0),
	COALESCE(SUM(estimated_cost_usd), 0)
FROM telemetry_events
WHERE `+filter, args...).Scan(
		&out.Events, &out.Requests, &out.Failures, &out.AvgDurationMS,
		&out.InputTokens, &out.CachedInputTokens, &out.CacheCreationTokens, &out.OutputTokens,
		&out.ReasoningOutputTokens, &out.TotalTokens, &out.EstimatedCostUSD,
	)
	return out, err
}

func (d *DB) FirstEventAt(ctx context.Context) (*time.Time, error) {
	return d.FirstEventAtBySource(ctx, "")
}

func (d *DB) FirstEventAtBySource(ctx context.Context, source string) (*time.Time, error) {
	var first sql.NullString
	filter, args := sourceFilter("", source)
	err := d.db.QueryRowContext(ctx, `SELECT MIN(ts) FROM telemetry_events WHERE `+filter, args...).Scan(&first)
	if err != nil {
		return nil, err
	}
	if !first.Valid {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, first.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func (d *DB) Series(ctx context.Context, since time.Time) ([]SeriesPoint, error) {
	return d.SeriesBySource(ctx, since, "")
}

func (d *DB) SeriesBySource(ctx context.Context, since time.Time, source string) ([]SeriesPoint, error) {
	since = startOfUTCDay(since)
	filter, args := sourceFilter(since.UTC().Format(time.RFC3339Nano), source)
	rows, err := d.db.QueryContext(ctx, `
SELECT
	strftime('%Y-%m-%d', ts),
	COUNT(*),
	COALESCE(SUM(CASE WHEN name IN ('codex.api_request', 'claude_code.api_request') THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(input_tokens), 0),
	COALESCE(SUM(cached_input_tokens), 0),
	COALESCE(SUM(cache_creation_tokens), 0),
	COALESCE(SUM(output_tokens), 0),
	COALESCE(SUM(reasoning_output_tokens), 0),
	COALESCE(SUM(total_tokens), 0),
	COALESCE(SUM(estimated_cost_usd), 0)
FROM telemetry_events
WHERE `+filter+`
GROUP BY 1
ORDER BY 1`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byBucket := make(map[string]SeriesPoint)
	for rows.Next() {
		var point SeriesPoint
		if err := rows.Scan(
			&point.Bucket, &point.Events, &point.Requests, &point.Failures,
			&point.InputTokens, &point.CachedInputTokens, &point.CacheCreationTokens, &point.OutputTokens,
			&point.ReasoningOutputTokens, &point.TotalTokens, &point.EstimatedCostUSD,
		); err != nil {
			return nil, err
		}
		byBucket[point.Bucket] = point
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	today := startOfUTCDay(time.Now().UTC())
	out := make([]SeriesPoint, 0)
	for day := since; !day.After(today); day = day.AddDate(0, 0, 1) {
		bucket := day.Format("2006-01-02")
		point, ok := byBucket[bucket]
		if !ok {
			point = SeriesPoint{Bucket: bucket}
		}
		out = append(out, point)
	}
	return out, nil
}

func startOfUTCDay(t time.Time) time.Time {
	year, month, day := t.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func (d *DB) ModelBreakdown(ctx context.Context, since time.Time) ([]BreakdownRow, error) {
	return d.ModelBreakdownBySource(ctx, since, "")
}

func (d *DB) ModelBreakdownBySource(ctx context.Context, since time.Time, source string) ([]BreakdownRow, error) {
	filter, args := sourceFilter(since.UTC().Format(time.RFC3339Nano), source)
	rows, err := d.db.QueryContext(ctx, `
SELECT COALESCE(NULLIF(source, ''), 'unknown'), COALESCE(NULLIF(model, ''), 'unknown'), COUNT(*), COALESCE(SUM(total_tokens), 0), COALESCE(SUM(estimated_cost_usd), 0)
FROM telemetry_events
WHERE `+filter+`
GROUP BY 1, 2
ORDER BY 5 DESC, 4 DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]BreakdownRow, 0)
	for rows.Next() {
		var row BreakdownRow
		if err := rows.Scan(&row.Source, &row.Model, &row.Events, &row.TotalTokens, &row.EstimatedCostUSD); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (d *DB) SourceBreakdown(ctx context.Context, since time.Time) ([]SourceBreakdownRow, error) {
	return d.SourceBreakdownBySource(ctx, since, "")
}

func (d *DB) SourceBreakdownBySource(ctx context.Context, since time.Time, source string) ([]SourceBreakdownRow, error) {
	filter, args := sourceFilter(since.UTC().Format(time.RFC3339Nano), source)
	rows, err := d.db.QueryContext(ctx, `
SELECT
	COALESCE(NULLIF(source, ''), 'unknown'),
	COUNT(*),
	COALESCE(SUM(CASE WHEN name IN ('codex.api_request', 'claude_code.api_request') THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(total_tokens), 0),
	COALESCE(SUM(estimated_cost_usd), 0)
FROM telemetry_events
WHERE `+filter+`
GROUP BY 1
ORDER BY 5 DESC, 4 DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SourceBreakdownRow, 0)
	for rows.Next() {
		var row SourceBreakdownRow
		if err := rows.Scan(&row.Source, &row.Events, &row.Requests, &row.TotalTokens, &row.EstimatedCostUSD); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (d *DB) IngestionHealth(ctx context.Context) (Health, error) {
	var out Health
	var last sql.NullString
	err := d.db.QueryRowContext(ctx, `
SELECT MAX(ts), COUNT(*), COALESCE(SUM(dropped_content_fields), 0)
FROM telemetry_events`).Scan(&last, &out.AcceptedEvents, &out.DroppedContentFields)
	if err != nil {
		return out, err
	}
	if last.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, last.String)
		if err != nil {
			return out, err
		}
		out.LastEventAt = &parsed
	}
	return out, nil
}

func (d *DB) Count(ctx context.Context) (int64, error) {
	var count int64
	err := d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM telemetry_events`).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return count, err
}

func defaultSource(source string) string {
	if source == "" {
		return "codex"
	}
	return source
}

func sourceFilter(since string, source string) (string, []any) {
	parts := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if since != "" {
		parts = append(parts, "ts >= ?")
		args = append(args, since)
	}
	if normalized := NormalizeSource(source); normalized != "" {
		parts = append(parts, "source = ?")
		args = append(args, normalized)
	}
	if len(parts) == 0 {
		return "1 = 1", args
	}
	return strings.Join(parts, " AND "), args
}

func NormalizeSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "", "all":
		return ""
	case "codex":
		return "codex"
	case "claude", "claude_code", "claude-code":
		return "claude-code"
	default:
		return ""
	}
}
