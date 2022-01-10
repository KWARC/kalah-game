-- -*- sql-product: sqlite; -*-

SELECT RANK() OVER (ORDER BY agent.score DESC),
       agent.id, agent.name, agent.author, agent.score, COUNT(agent.id)
FROM agent
CROSS JOIN game ON agent.id == game.north OR agent.id == game.south
GROUP BY agent.id
ORDER BY agent.score DESC
LIMIT ?2
OFFSET ?1 * ?2;
