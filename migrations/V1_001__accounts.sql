CREATE TABLE accounts (
    account_id text PRIMARY KEY,
    balance numeric(20, 2) NOT NULL CHECK (balance >= 0)
);
