-- -*- sql-product: sqlite; -*-

CREATE TABLE IF NOT EXISTS move (
	comment TEXT,
	agent REFERENCES agent(id),
	side BOOLEAN,		-- See Side in board.go
	game REFERENCES game(id),
	played DATETIME,
	choice INT
);
