-- -*- sql-product: sqlite; -*-

CREATE TABLE IF NOT EXISTS move (
	comment TEXT,
	agent REFERENCES agent(id) ON DELETE CASCADE NOT NULL,
	side BOOLEAN NOT NULL,	-- See Side in board.go
	game REFERENCES game(id) ON DELETE CASCADE NOT NULL,
	played DATETIME NOT NULL,
	choice INT NOT NULL
);
