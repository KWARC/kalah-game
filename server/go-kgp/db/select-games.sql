-- -*- sql-product: sqlite; -*-

SELECT game.id, game.size, game.init, game.north, game.south, game.outcome,
       COUNT(move.game)
FROM game INNER JOIN move ON game.id = move.game
GROUP BY game.id
ORDER BY MAX(move.played) DESC
LIMIT 25
OFFSET ?1 * 100;
