-- -*- sql-product: sqlite; -*-

SELECT game.id, game.size, game.init, game.north, game.south, game.state,
       COUNT(move.game), MAX(move.played)
FROM game INNER JOIN move ON game.id = move.game
GROUP BY game.id;
