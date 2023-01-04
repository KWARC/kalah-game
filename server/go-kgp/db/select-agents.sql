-- -*- sql-product: sqlite; -*-

SELECT agent.id, agent.name, agent.author, COUNT(agent.id)
FROM agent
CROSS JOIN game ON agent.id == game.north OR agent.id == game.south
GROUP BY agent.id
ORDER BY agent.id DESC
LIMIT 50
OFFSET ?1 * 50;
