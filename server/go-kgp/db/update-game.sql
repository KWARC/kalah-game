-- -*- sql-product: sqlite; -*-

UPDATE OR IGNORE game
SET outcome = ?
WHERE id = ?;
