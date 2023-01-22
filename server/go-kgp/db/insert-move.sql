-- -*- sql-product: sqlite; -*-

INSERT INTO move(game, agent, side, choice, comment, played)
VALUES (?, ?, ?, ?, ?, strftime("%Y-%m-%d %H:%M:%S.000000000+00:00"));
