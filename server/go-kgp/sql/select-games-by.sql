-- -*- sql-product: sqlite; -*-

SELECT id, size, init, north, south, outcome, start, end
FROM game
WHERE (north == ?1 OR south == ?1) AND end IS NOT NULL
ORDER BY start
LIMIT 100
OFFSET ? * 100;
