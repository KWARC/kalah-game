-- -*- sql-product: sqlite; -*-

SELECT id, name, score
FROM agent
ORDER BY score DESC
LIMIT 100
OFFSET ? * 100

