-- -*- sql-product: sqlite; -*-

CREATE TABLE IF NOT EXISTS game (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	north REFERENCES agent(id),
	south REFERENCES agent(id),
	result INTEGER,
	start DATETIME
);
