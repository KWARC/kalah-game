-- -*- sql-product: sqlite; -*-

INSERT INTO agent(token, name, descr, author, score)
VALUES (?1, ?2, ?3, ?4, 1000)
ON CONFLICT (token)
DO UPDATE SET name = ?2, descr = ?3, author = ?4, score = ?5;
