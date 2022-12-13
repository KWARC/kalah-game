-- -*- sql-product: sqlite; -*-

CREATE TABLE IF NOT EXISTS game (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	size INTEGER CHECK(size > 0) NOT NULL,
	init INTEGER CHECK(init > 0) NOT NULL,
	north REFERENCES agent(id) ON DELETE CASCADE,
	south REFERENCES agent(id) ON DELETE CASCADE,
	state TEXT CHECK(state IN ("o", "nw", "sw", "u", "nr", "sr", "a"))
);
