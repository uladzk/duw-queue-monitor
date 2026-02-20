-- +goose Up
-- +goose StatementBegin
ALTER TABLE queue_daily_stats RENAME COLUMN registered_tickets TO taken_tickets;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE queue_daily_stats RENAME COLUMN taken_tickets TO registered_tickets;
-- +goose StatementEnd
