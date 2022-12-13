-- -*- sql-product: sqlite; -*-

-- If the server stopped and the database had an ongoing game, we will
-- regard it as aborted.
UPDATE OR IGNORE game
SET state = "a"
WHERE state = "o";
