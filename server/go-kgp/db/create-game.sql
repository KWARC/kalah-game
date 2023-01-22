-- -*- sql-product: sqlite; -*-

CREATE TABLE IF NOT EXISTS game (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	size INTEGER CHECK(size > 0) NOT NULL,
	init INTEGER CHECK(init > 0) NOT NULL,
	north REFERENCES agent(id) ON DELETE CASCADE NOT NULL,
	south REFERENCES agent(id) ON DELETE CASCADE NOT NULL,
	state TEXT CHECK(state IN ("o", "nw", "sw", "u", "nr", "sr", "a"))
);
