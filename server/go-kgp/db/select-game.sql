-- -*- sql-product: sqlite; -*-

SELECT game.id, game.size, game.init, game.north, game.south, game.state,
       COUNT(move.game)
FROM game LEFT JOIN move ON game.id = move.game
WHERE game.id = ?;
