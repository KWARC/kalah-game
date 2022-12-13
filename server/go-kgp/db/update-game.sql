-- -*- sql-product: sqlite; -*-

UPDATE OR IGNORE game
SET state = ?
WHERE id = ?;
