-- name: UpsertDailyStats :exec
INSERT INTO queue_daily_stats (queue_id, queue_name, date, total_tickets_available, taken_tickets, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW())
ON CONFLICT (queue_id, date)
DO UPDATE SET
    total_tickets_available = EXCLUDED.total_tickets_available,
    taken_tickets = EXCLUDED.taken_tickets,
    queue_name = EXCLUDED.queue_name,
    updated_at = NOW();

-- name: GetStatsByDateRange :many
SELECT id, queue_id, queue_name, date, total_tickets_available, taken_tickets, created_at, updated_at
FROM queue_daily_stats
WHERE queue_id = $1 AND date BETWEEN $2 AND $3
ORDER BY date ASC;
