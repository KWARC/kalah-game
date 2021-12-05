-- -*- sql-product: sqlite; -*-

SELECT agent.id, agent.name, agent.score, COUNT(agent.id)
FROM agent
CROSS JOIN game ON agent.id == game.north OR agent.id == game.south
GROUP BY agent.id
ORDER BY agent.score DESC
LIMIT ?2
OFFSET ?1 * ?2;
