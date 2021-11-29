-- -*- sql-product: sqlite; -*-

SELECT agent, side, comment, choice
FROM move
WHERE game = ?
ORDER BY played
