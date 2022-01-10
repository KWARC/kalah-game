-- -*- sql-product: sqlite; -*-

SELECT game.id, game.size, game.init, game.north, game.south, game.outcome,
       COUNT(move.game)
FROM game INNER JOIN move ON game.id = move.game
WHERE game.id = ?;
