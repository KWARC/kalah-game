-- -*- sql-product: sqlite; -*-

INSERT INTO move(comment, agent, game, played, choice)
VALUES (?, ?, ?, DATETIME('now'), ?);
