-- -*- sql-product: sqlite; -*-

SELECT name, descr, author, COUNT(1)
FROM agent
LEFT JOIN game ON agent.id == game.north OR agent.id == game.south
WHERE agent.id = ?
GROUP BY agent.id;
