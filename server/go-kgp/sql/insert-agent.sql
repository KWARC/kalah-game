-- -*- sql-product: sqlite; -*-

INSERT INTO agent(token, name, descr, score)
VALUES (?, ?, ?, 1000)
ON CONFLICT (token)
DO UPDATE SET name = ?, descr = ?, score = ?
