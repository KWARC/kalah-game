-- -*- sql-product: sqlite; -*-

SELECT w.name, w.id, l.name, l.id
FROM game
JOIN agent AS w ON ((w.id == south AND state == "sw")
     	        OR  (w.id == north AND state == "nw"))
JOIN agent AS l ON ((l.id == north AND state == "sw")
     	        OR  (l.id == south AND state == "nw"))
GROUP BY w.id, l.id;
