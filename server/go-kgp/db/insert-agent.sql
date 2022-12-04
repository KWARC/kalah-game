-- -*- sql-product: sqlite; -*-

INSERT INTO agent(token, name, descr, author)
VALUES (?1, ?2, ?3, ?4)
ON CONFLICT (token)
DO UPDATE SET name = ?2, descr = ?3, author = ?4;
