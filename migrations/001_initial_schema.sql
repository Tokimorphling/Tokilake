-- SQLite migration file
CREATE TABLE IF NOT EXISTS clients(
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    "type" TEXT NOT NULL,
    namespace TEXT NOT NULL UNIQUE,
    api_base TEXT,
    api_key TEXT,
    public BOOLEAN DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS models(
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    client_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    FOREIGN KEY(client_id) REFERENCES clients(id)
);