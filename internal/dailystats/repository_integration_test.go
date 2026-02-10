package dailystats

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const migrationsDir = "../../db/migrations"

func initPostgresContainer(ctx context.Context, t *testing.T) (testcontainers.Container, *sql.DB) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:17-alpine",
		Name:         "dailystats-postgres-integration-test",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "duw_stats_test",
			"POSTGRES_USER":     "test_user",
			"POSTGRES_PASSWORD": "test_pass",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
	}

	pgC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}

	endpoint, err := pgC.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("Failed to get PostgreSQL endpoint: %v", err)
	}

	connStr := "postgres://test_user:test_pass@" + endpoint + "/duw_stats_test?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to open database connection: %v", err)
	}

	if err := goose.Up(db, migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	return pgC, db
}

func TestSaveDailyStats_WhenNewRecord_InsertsSuccessfully(t *testing.T) {
	// Arrange
	ctx := context.Background()
	pgC, db := initPostgresContainer(ctx, t)
	defer testcontainers.CleanupContainer(t, pgC)
	defer db.Close()

	sut := NewRepository(db)
	date := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)

	// Act
	err := sut.SaveDailyStats(ctx, 24, "Odbior karty", date, 42, 50)

	// Assert
	if err != nil {
		t.Fatalf("Expected successful insert, but got error: %v", err)
	}

	result, err := sut.GetByDate(ctx, 24, date)
	if err != nil {
		t.Fatalf("Expected successful get, but got error: %v", err)
	}

	if result.QueueID != 24 {
		t.Errorf("Expected QueueID 24, got %d", result.QueueID)
	}
	if result.QueueName != "Odbior karty" {
		t.Errorf("Expected QueueName 'Odbior karty', got '%s'", result.QueueName)
	}
	if result.TicketsServed != 42 {
		t.Errorf("Expected TicketsServed 42, got %d", result.TicketsServed)
	}
	if result.RegisteredTickets != 50 {
		t.Errorf("Expected RegisteredTickets 50, got %d", result.RegisteredTickets)
	}
}

func TestSaveDailyStats_WhenRecordExists_UpsertsSuccessfully(t *testing.T) {
	// Arrange
	ctx := context.Background()
	pgC, db := initPostgresContainer(ctx, t)
	defer testcontainers.CleanupContainer(t, pgC)
	defer db.Close()

	sut := NewRepository(db)
	date := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)

	err := sut.SaveDailyStats(ctx, 24, "Odbior karty", date, 42, 50)
	if err != nil {
		t.Fatalf("Failed to insert initial record: %v", err)
	}

	// Act
	err = sut.SaveDailyStats(ctx, 24, "Odbior karty", date, 60, 70)

	// Assert
	if err != nil {
		t.Fatalf("Expected successful upsert, but got error: %v", err)
	}

	result, err := sut.GetByDate(ctx, 24, date)
	if err != nil {
		t.Fatalf("Expected successful get, but got error: %v", err)
	}

	if result.TicketsServed != 60 {
		t.Errorf("Expected TicketsServed 60 after upsert, got %d", result.TicketsServed)
	}
	if result.RegisteredTickets != 70 {
		t.Errorf("Expected RegisteredTickets 70 after upsert, got %d", result.RegisteredTickets)
	}
}

func TestGetByDateRange_WhenMultipleRecords_ReturnsInDateOrder(t *testing.T) {
	// Arrange
	ctx := context.Background()
	pgC, db := initPostgresContainer(ctx, t)
	defer testcontainers.CleanupContainer(t, pgC)
	defer db.Close()

	sut := NewRepository(db)

	dates := []time.Time{
		time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 11, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC),
	}

	for i, date := range dates {
		if err := sut.SaveDailyStats(ctx, 24, "Odbior karty", date, (i+1)*10, (i+1)*15); err != nil {
			t.Fatalf("Failed to insert record for date %v: %v", date, err)
		}
	}

	// Act
	results, err := sut.GetByDateRange(ctx, 24, dates[0], dates[2])

	// Assert
	if err != nil {
		t.Fatalf("Expected successful query, but got error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	for i, result := range results {
		expectedServed := int32((i + 1) * 10)
		if result.TicketsServed != expectedServed {
			t.Errorf("Result %d: expected TicketsServed %d, got %d", i, expectedServed, result.TicketsServed)
		}
		if !result.Date.Equal(dates[i]) {
			t.Errorf("Result %d: expected date %v, got %v", i, dates[i], result.Date)
		}
	}
}
