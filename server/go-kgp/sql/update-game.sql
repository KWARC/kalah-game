-- -*- sql-product: sqlite; -*-

UPDATE game
SET outcome = ?, end = DATETIME('now')
WHERE id = ?
