-- -*- sql-product: sqlite; -*-

CREATE TABLE IF NOT EXISTS move (
	comment TEXT,
	agent REFERENCES agent(id),
	game REFERENCES game(id),
	played DATETIME,
	choice INT
);
