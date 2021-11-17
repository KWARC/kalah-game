-- -*- sql-product: sqlite; -*-

CREATE TABLE IF NOT EXISTS agent (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	token TEXT UNIQUE,
	name TEXT,
	descr TEXT,
	score REAL
);
