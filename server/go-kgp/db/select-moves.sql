-- -*- sql-product: sqlite; -*-

SELECT side, comment, choice, played
FROM move
WHERE game = ?
ORDER BY played;
