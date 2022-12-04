-- -*- sql-product: sqlite; -*-

SELECT id, name, descr FROM agent WHERE token = ?;
