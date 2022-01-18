-- -*- sql-product: sqlite; -*-

INSERT INTO tournament(name, start) VALUES (?, DATETIME('now'));
