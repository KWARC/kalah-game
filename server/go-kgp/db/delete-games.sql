-- -*- sql-product: sqlite; -*-

WITH forget AS (SELECT move.game FROM move GROUP BY move.game
     	        HAVING MAX(move.played)
		     < strftime("%Y-%m-%d %H:%M:%S.000000000+00:00",
		                "now", "-7 day"))
DELETE FROM game WHERE game.id IN forget;
