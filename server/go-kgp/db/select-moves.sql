-- -*- sql-product: sqlite; -*-

SELECT side, comment, choice
FROM move
WHERE game = ?
ORDER BY played;
