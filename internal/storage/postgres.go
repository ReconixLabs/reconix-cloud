package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reconix-cloud/internal/model"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct{ DB *pgxpool.Pool }

func OpenPostgres(ctx context.Context, url string) (*Postgres, error) {
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if _, err = db.Exec(ctx, `CREATE TABLE IF NOT EXISTS scans (
		id TEXT PRIMARY KEY, target TEXT NOT NULL, profile TEXT NOT NULL, status TEXT NOT NULL,
		stage TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL, started_at TIMESTAMPTZ,
		completed_at TIMESTAMPTZ, updated_at TIMESTAMPTZ NOT NULL, error TEXT NOT NULL DEFAULT '',
		raw_result JSONB, normalized_result JSONB
	); CREATE INDEX IF NOT EXISTS scans_status_created_idx ON scans(status, created_at)`); err != nil {
		db.Close()
		return nil, err
	}
	return &Postgres{DB: db}, nil
}
func (s *Postgres) Create(ctx context.Context, scan model.Scan) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO scans (id,target,profile,status,stage,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, scan.ID, scan.Target, scan.Profile, scan.Status, scan.Stage, scan.CreatedAt, scan.UpdatedAt)
	return err
}
func (s *Postgres) Get(ctx context.Context, id string) (model.Scan, error) {
	var x model.Scan
	err := s.DB.QueryRow(ctx, `SELECT id,target,profile,status,stage,created_at,started_at,completed_at,updated_at,error FROM scans WHERE id=$1`, id).Scan(&x.ID, &x.Target, &x.Profile, &x.Status, &x.Stage, &x.CreatedAt, &x.StartedAt, &x.CompletedAt, &x.UpdatedAt, &x.Error)
	return x, err
}
func (s *Postgres) List(ctx context.Context) ([]model.Scan, error) {
	rows, err := s.DB.Query(ctx, `SELECT id,target,profile,status,stage,created_at,started_at,completed_at,updated_at,error FROM scans ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Scan
	for rows.Next() {
		var x model.Scan
		if err := rows.Scan(&x.ID, &x.Target, &x.Profile, &x.Status, &x.Stage, &x.CreatedAt, &x.StartedAt, &x.CompletedAt, &x.UpdatedAt, &x.Error); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Postgres) Claim(ctx context.Context, id string) error {
	now := time.Now().UTC()
	tag, err := s.DB.Exec(ctx, `UPDATE scans SET status='starting',stage='reconix',started_at=$2,updated_at=$2 WHERE id=$1 AND status='queued'`, id, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("scan is not queued")
	}
	return nil
}
func (s *Postgres) ClaimNext(ctx context.Context, staleAfter time.Duration) (model.Scan, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return model.Scan{}, err
	}
	defer tx.Rollback(ctx)
	seconds := int64(staleAfter / time.Second)
	row := tx.QueryRow(ctx, `
		WITH candidate AS (
			SELECT id FROM scans
			WHERE status = 'queued'
			   OR (status IN ('starting', 'running') AND updated_at < NOW() - ($1 * INTERVAL '1 second'))
			ORDER BY created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE scans AS s
		SET status = 'starting', stage = 'reconix', started_at = NOW(), updated_at = NOW()
		FROM candidate
		WHERE s.id = candidate.id
		RETURNING s.id, s.target, s.profile, s.status, s.stage, s.created_at,
		          s.started_at, s.completed_at, s.updated_at, s.error`, seconds)
	var scan model.Scan
	if err := row.Scan(&scan.ID, &scan.Target, &scan.Profile, &scan.Status, &scan.Stage, &scan.CreatedAt, &scan.StartedAt, &scan.CompletedAt, &scan.UpdatedAt, &scan.Error); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Scan{}, ErrNoJob
		}
		return model.Scan{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.Scan{}, err
	}
	return scan, nil
}
func (s *Postgres) Update(ctx context.Context, scan model.Scan, raw []byte, result *model.NormalizedResult) error {
	var normalized []byte
	if result != nil {
		normalized, _ = json.Marshal(result)
	}
	_, err := s.DB.Exec(ctx, `UPDATE scans SET status=$2,stage=$3,started_at=$4,completed_at=$5,updated_at=$6,error=$7,raw_result=$8,normalized_result=$9 WHERE id=$1`, scan.ID, scan.Status, scan.Stage, scan.StartedAt, scan.CompletedAt, scan.UpdatedAt, scan.Error, raw, normalized)
	return err
}
func (s *Postgres) Cancel(ctx context.Context, id string) error {
	tag, err := s.DB.Exec(ctx, `UPDATE scans SET status='cancelled',updated_at=NOW() WHERE id=$1 AND status IN ('queued','starting','running')`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("scan cannot be cancelled")
	}
	return nil
}
func (s *Postgres) Result(id string) (model.NormalizedResult, error) {
	var data []byte
	err := s.DB.QueryRow(context.Background(), `SELECT normalized_result FROM scans WHERE id=$1`, id).Scan(&data)
	if err != nil {
		return model.NormalizedResult{}, err
	}
	var result model.NormalizedResult
	err = json.Unmarshal(data, &result)
	return result, err
}
