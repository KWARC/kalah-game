-- -*- sql-product: sqlite; -*-

CREATE TABLE IF NOT EXISTS game (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	size INTEGER,
	init INTEGER,
	north REFERENCES agent(id),
	south REFERENCES agent(id),
	state TEXT CHECK(state IN ("o", "nw", "sw", "u", "nr", "sr", "a"))
);
