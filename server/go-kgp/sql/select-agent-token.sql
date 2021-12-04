-- -*- sql-product: sqlite; -*-

SELECT id, name, descr, score FROM agent WHERE token = ?;
