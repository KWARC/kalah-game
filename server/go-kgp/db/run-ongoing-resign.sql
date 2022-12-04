-- -*- sql-product: sqlite; -*-

-- If the server stopped and the database had an ongoing game, we will
-- regard it as a resignation.
UPDATE OR IGNORE game
SET outcome = 4
WHERE outcome = 0;
