-- -*- sql-product: sqlite; -*-

INSERT INTO game(size, init, north, south, start)
VALUES (?, ?, ?, ?, DATETIME('now'));
