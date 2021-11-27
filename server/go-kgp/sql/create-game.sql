-- -*- sql-product: sqlite; -*-

CREATE TABLE IF NOT EXISTS game (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	north REFERENCES agent(id),
	south REFERENCES agent(id),
	outcome INTEGER,		-- See Outcome in game.go
	start DATETIME,
	end DATETIME
);
