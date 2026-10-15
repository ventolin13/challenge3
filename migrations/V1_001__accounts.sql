CREATE TABLE accounts (
    account_id text NOT NULL,
    balance numeric(20, 2) NOT NULL,
    CONSTRAINT accounts_account_id_unique PRIMARY KEY (account_id),
    CONSTRAINT accounts_balance_non_negative CHECK (balance >= 0)
);
