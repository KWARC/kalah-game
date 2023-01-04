-- -*- sql-product: sqlite; -*-

SELECT game.id, game.size, game.init, game.north, game.south, game.state,
       COUNT(move.game), MAX(move.played)
FROM game INNER JOIN move ON game.id = move.game
WHERE game.north == ?1 OR game.south == ?1
GROUP BY game.id
ORDER BY MAX(move.played) DESC
LIMIT 50
OFFSET ?2 * 100;
