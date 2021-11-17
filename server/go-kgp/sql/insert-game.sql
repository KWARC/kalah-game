-- -*- sql-product: sqlite; -*-

INSERT INTO game(north, south, start)
VALUES (?, ?, DATETIME('now'));
