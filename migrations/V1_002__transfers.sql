CREATE TABLE transfers (
    transfer_id uuid NOT NULL,
    from_account_id text NOT NULL,
    to_account_id text NOT NULL,
    amount numeric(20, 2) NOT NULL,
    status text NOT NULL,
    reason text,
    created_at timestamptz NOT NULL,
    CONSTRAINT transfers_transfer_id_unique PRIMARY KEY (transfer_id),
    CONSTRAINT transfers_amount_positive CHECK (amount > 0)
);
