-- +goose Up
-- +goose StatementBegin
ALTER TABLE queue_daily_stats RENAME COLUMN tickets_served TO total_tickets_available;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE queue_daily_stats RENAME COLUMN total_tickets_available TO tickets_served;
-- +goose StatementEnd
