CREATE TABLE IF NOT EXISTS scans (
    id TEXT PRIMARY KEY,
    target TEXT NOT NULL,
    profile TEXT NOT NULL,
    status TEXT NOT NULL,
    stage TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL,
    error TEXT NOT NULL DEFAULT '',
    raw_result JSONB,
    normalized_result JSONB
);
CREATE INDEX IF NOT EXISTS scans_status_created_idx ON scans(status, created_at);
