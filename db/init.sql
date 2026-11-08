CREATE TABLE tickets (
    id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    description text NOT NULL,
    status text NOT NULL DEFAULT 'new'
);
