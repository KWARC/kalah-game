-- -*- sql-product: sqlite; -*-

SELECT agent, comment, choice
FROM move
WHERE game = ?
ORDER BY played;
