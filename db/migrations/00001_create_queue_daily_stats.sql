-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS queue_daily_stats (
    id BIGSERIAL PRIMARY KEY,
    queue_id INTEGER NOT NULL,
    queue_name VARCHAR(255) NOT NULL,
    date DATE NOT NULL,
    tickets_served INTEGER NOT NULL DEFAULT 0,
    registered_tickets INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT queue_daily_stats_queue_date_unique UNIQUE (queue_id, date)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS queue_daily_stats;
-- +goose StatementEnd
