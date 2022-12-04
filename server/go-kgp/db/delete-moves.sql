-- -*- sql-product: sqlite; -*-

DELETE FROM move
WHERE played < date('weekday 1', '-1 week');
