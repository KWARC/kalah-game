-- -*- sql-product: sqlite; -*-

SELECT game.id, game.size, game.init, game.north, game.south, game.state,
       COUNT(move.game)
FROM game INNER JOIN move ON game.id = move.game
WHERE game.north == ?1 OR game.south == ?1
GROUP BY game.id
ORDER BY MAX(move.played) DESC
LIMIT 100
OFFSET ?2 * 100;
