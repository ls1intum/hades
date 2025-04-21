package persister

import (
	"context"
	"database/sql"
	_ "embed"
	"log/slog"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/google/uuid"
	"github.com/ls1intum/hades/benchmarkingSuite/persister/model"
)

const file string = "benchmark.db"

// Persister interface
// This interface is used to store the job and the result of the job
// This implementation allows to abstract the concrete storage mechanism
type Persister interface {
	StoreJob(uuid uuid.UUID, time time.Time, executor string)
	StoreResult(uuid uuid.UUID, startTime time.Time, endTime time.Time)
}

// DBPersister is a concrete implementation of the Persister interface
// It uses a SQLite database to store the job and the result
type DBPersister struct {
	db      *sql.DB
	queries *model.Queries
}

//go:embed schema.sql
var ddl string

func NewDBPersister() DBPersister {
	ctx := context.Background()

	// Open the database
	db, err := sql.Open("sqlite3", file)
	if err != nil {
		slog.Error("Error while opening DB", slog.Any("error", err))
		// Panic if the database cannot be opened
		panic(err)
	}

	// Create the table
	if _, err := db.ExecContext(ctx, ddl); err != nil {
		slog.Debug("Error while creating table", slog.Any("error", err))
	}

	queries := model.New(db)

	return DBPersister{
		db:      db,
		queries: queries,
	}
}

func (d DBPersister) StoreJob(uuid uuid.UUID, time time.Time, executor string) {

	d.queries.StoreScheduledJob(context.Background(), model.StoreScheduledJobParams{
		ID:           uuid,
		CreationTime: time.String(),
		Executor:     executor,
	})
}

func (d DBPersister) StoreResult(uuid uuid.UUID, startTime time.Time, endTime time.Time) {
	d.queries.StoreJobResult(context.Background(), model.StoreJobResultParams{
		ID:        uuid,
		StartTime: startTime.String(),
		EndTime:   endTime.String(),
	})

}
