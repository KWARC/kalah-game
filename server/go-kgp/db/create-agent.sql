-- -*- sql-product: sqlite; -*-

CREATE TABLE IF NOT EXISTS agent (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	token BLOB UNIQUE,
	name TEXT,
	descr TEXT,
	author TEXT
);
