CREATE TABLE scheduled_job (
  id        uuid  PRIMARY KEY,
  creation_time      text    NOT NULL,
  executor  text    NOT NULL, 
  metadata  jsonb
);

CREATE TABLE job_results (
  id        uuid  PRIMARY KEY,
  start_time      text    NOT NULL,
  end_time        text    NOT NULL
);