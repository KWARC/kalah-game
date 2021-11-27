-- -*- sql-product: sqlite; -*-

SELECT id, north, south, outcome, start, end
FROM game
WHERE id = ?
