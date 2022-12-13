-- -*- sql-product: sqlite; -*-

DELETE FROM move
WHERE played < strftime("%Y-%m-%d %H:%M:%S.000000000+00:00", "now", "-1 week");
