-- -*- sql-product: sqlite; -*-

INSERT INTO move(comment, agent, game, played)
VALUES (?, ?, ?, DATETIME('now'));
