CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id UUID NOT NULL,
    aggregate_version BIGINT NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_by VARCHAR(100),
    locked_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    last_error TEXT,
    CONSTRAINT outbox_status_check CHECK (status IN ('PENDING', 'PROCESSING', 'PROCESSED', 'DEAD')),
    CONSTRAINT outbox_version_check CHECK (aggregate_version >= 1),
    CONSTRAINT outbox_attempts_check CHECK (attempts >= 0),
    CONSTRAINT outbox_event_version_unique UNIQUE (aggregate_type, aggregate_id, aggregate_version)
);

CREATE INDEX outbox_pending_events_idx ON outbox_events (available_at, created_at) WHERE status = 'PENDING';

CREATE INDEX outbox_expired_leases_idx ON outbox_events (locked_until) WHERE status = 'PROCESSING';

