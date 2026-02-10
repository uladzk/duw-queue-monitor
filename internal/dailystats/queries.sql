-- name: UpsertDailyStats :exec
INSERT INTO queue_daily_stats (queue_id, queue_name, date, tickets_served, registered_tickets, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW())
ON CONFLICT (queue_id, date)
DO UPDATE SET
    tickets_served = EXCLUDED.tickets_served,
    registered_tickets = EXCLUDED.registered_tickets,
    queue_name = EXCLUDED.queue_name,
    updated_at = NOW();

-- name: GetStatsByDateRange :many
SELECT id, queue_id, queue_name, date, tickets_served, registered_tickets, created_at, updated_at
FROM queue_daily_stats
WHERE queue_id = $1 AND date BETWEEN $2 AND $3
ORDER BY date ASC;
