CREATE TABLE IF NOT EXISTS ticket_status_history (
    event_id bigint PRIMARY KEY,
    ticket_id integer NOT NULL REFERENCES tickets(id),
    old_status text NOT NULL,
    new_status text NOT NULL,
    occurred_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS ticket_status_history_ticket_id_event_id_idx
    ON ticket_status_history (ticket_id, event_id);
