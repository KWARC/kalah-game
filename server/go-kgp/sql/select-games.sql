-- -*- sql-product: sqlite; -*-

SELECT id, size, init, north, south, outcome, start, end
FROM game
WHERE end IS NOT NULL
ORDER BY start DESC
LIMIT ?2
OFFSET ?1 * ?2;
