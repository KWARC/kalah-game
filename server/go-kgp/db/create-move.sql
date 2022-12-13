-- -*- sql-product: sqlite; -*-

CREATE TABLE IF NOT EXISTS move (
	comment TEXT,
	agent REFERENCES agent(id) ON DELETE CASCADE,
	side BOOLEAN,		-- See Side in board.go
	game REFERENCES game(id) ON DELETE CASCADE,
	played DATETIME,
	choice INT
);
