-- -*- sql-product: sqlite; -*-

SELECT north, south, result, start
FROM game
WHERE id = ?
