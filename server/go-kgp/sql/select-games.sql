-- -*- sql-product: sqlite; -*-

SELECT id, north, south, result, start
FROM game
ORDER BY start
LIMIT 100
OFFSET ? * 100

