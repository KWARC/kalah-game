-- -*- sql-product: sqlite; -*-

CREATE TABLE IF NOT EXISTS score (
       id INTEGER PRIMARY KEY AUTOINCREMENT,
       agent      REFERENCES agent(id),
       game       REFERENCES game(id),
       tournament REFERENCES tournament(id),
       score REAL
);
