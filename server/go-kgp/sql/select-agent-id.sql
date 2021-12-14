-- -*- sql-product: sqlite; -*-

SELECT name, descr, author, score FROM agent WHERE id = ?;
