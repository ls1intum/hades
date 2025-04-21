-- name: StoreScheduledJobWithMetadata :one
INSERT INTO scheduled_job (
  id, creation_time, executor, metadata
) VALUES (
  ?, ?, ?, ?
)
RETURNING *;

-- name: StoreScheduledJob :one
INSERT INTO scheduled_job (
  id, creation_time, executor
) VALUES (
  ?, ?, ?
)
RETURNING *;

-- name: StoreJobResult :one
INSERT INTO job_results (
  id, start_time, end_time
) VALUES (
  ?, ?, ?
)
RETURNING *;