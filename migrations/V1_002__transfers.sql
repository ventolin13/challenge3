CREATE TABLE transfers (
    transfer_id uuid PRIMARY KEY,
    from_account_id text NOT NULL,
    to_account_id text NOT NULL,
    amount numeric(20, 2) NOT NULL CHECK (amount > 0),
    status text NOT NULL,
    reason text,
    created_at timestamptz NOT NULL
);
