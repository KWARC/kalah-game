-- -*- sql-product: sqlite; -*-

CREATE TABLE IF NOT EXISTS score (
       id INTEGER PRIMARY KEY AUTOINCREMENT,
       agent      REFERENCES agent(id) ON DELETE CASCADE,
       game       REFERENCES game(id) ON DELETE CASCADE,
       tournament REFERENCES tournament(id) ON DELETE CASCADE,
       score REAL
);
