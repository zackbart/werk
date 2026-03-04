package db

const schema = `
CREATE TABLE IF NOT EXISTS tasks (
  id          TEXT PRIMARY KEY,
  parent_id   TEXT REFERENCES tasks(id),
  type        TEXT NOT NULL CHECK(type IN ('epic','task','subtask')),
  title       TEXT NOT NULL,
  status      TEXT NOT NULL DEFAULT 'open'
                CHECK(status IN ('open','in_progress','blocked','done')),
  priority    INTEGER NOT NULL DEFAULT 2
                CHECK(priority BETWEEN 0 AND 4),
  notes       TEXT,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME,
  closed_at   DATETIME
);

CREATE TABLE IF NOT EXISTS dependencies (
  upstream_id   TEXT NOT NULL REFERENCES tasks(id),
  downstream_id TEXT NOT NULL REFERENCES tasks(id),
  PRIMARY KEY (upstream_id, downstream_id),
  CHECK (upstream_id != downstream_id)
);

CREATE TABLE IF NOT EXISTS audit (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id     TEXT NOT NULL REFERENCES tasks(id),
  field       TEXT NOT NULL,
  old_value   TEXT,
  new_value   TEXT,
  changed_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  changed_by  TEXT NOT NULL CHECK(changed_by IN ('agent','human'))
);

CREATE TABLE IF NOT EXISTS decisions (
  id          TEXT PRIMARY KEY,
  summary     TEXT NOT NULL,
  rationale   TEXT,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_by  TEXT NOT NULL CHECK(created_by IN ('agent','human'))
);

CREATE TABLE IF NOT EXISTS sessions (
  id            TEXT PRIMARY KEY,
  started_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  ended_at      DATETIME,
  summary       TEXT,
  tasks_touched TEXT DEFAULT '[]'
);
`

func (d *DB) Migrate() error {
	_, err := d.conn.Exec(schema)
	return err
}
