-- -*- sql-product: sqlite; -*-

INSERT INTO move(game, agent, side, choice, comment, played)
VALUES (?, ?, ?, ?, ?, ?);
