-- -*- sql-product: sqlite; -*-

SELECT id, size, init, north, south, outcome, start, end
FROM game
WHERE id = ?
