-- -*- sql-product: sqlite; -*-

CREATE TABLE IF NOT EXISTS tournament (
       id INTEGER PRIMARY KEY AUTOINCREMENT,
       name TEXT,
       start TIMESTAMP
);
