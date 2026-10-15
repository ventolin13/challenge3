INSERT INTO accounts (account_id, balance)
VALUES ('acc_1', 1000.00),
       ('acc_2', 1000.00),
       ('acc_3', 1000.00),
       ('acc_4', 1000.00),
       ('acc_5', 1000.00),
       ('acc_6', 1000.00),
       ('acc_7', 1000.00),
       ('acc_8', 1000.00),
       ('acc_9', 1000.00),
       ('acc_10', 1000.00)
ON CONFLICT (account_id) DO NOTHING;
