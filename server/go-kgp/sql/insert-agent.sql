-- -*- sql-product: sqlite; -*-

INSERT INTO agent(token, name, descr, score)
VALUES (?1, ?2, ?3, 1000)
ON CONFLICT (token)
DO UPDATE SET name = ?2, descr = ?3, author = ?4, score = ?5;
