CREATE TABLE IF NOT EXISTS ticket_outbox (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ticket_id integer NOT NULL REFERENCES tickets(id),
    old_status text NOT NULL,
    new_status text NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now()
);
