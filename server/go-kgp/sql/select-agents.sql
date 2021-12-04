-- -*- sql-product: sqlite; -*-

SELECT id, name, score
FROM agent
ORDER BY score DESC
LIMIT ?2
OFFSET ?1 * ?2;
