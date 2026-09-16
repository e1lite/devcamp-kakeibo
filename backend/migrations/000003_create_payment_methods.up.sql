-- 3.10 payment_methods: 決済手段のマスタ。
CREATE TABLE payment_methods (
    id         bigserial    PRIMARY KEY,
    user_id    bigint       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       varchar(100) NOT NULL,
    kind       varchar(20)  NOT NULL,
    issuer     varchar(100),
    card_last4 varchar(4),
    is_active  boolean      NOT NULL DEFAULT true,
    created_at timestamptz  NOT NULL DEFAULT now(),
    updated_at timestamptz  NOT NULL DEFAULT now(),

    CONSTRAINT uq_payment_methods_user_name UNIQUE (user_id, name),
    CONSTRAINT ck_payment_methods_kind
        CHECK (kind IN ('credit_card', 'qr', 'bank', 'cash', 'other'))
);

-- メール中の下 4 桁から決済手段を特定する（Step 8 以降のパース処理で使う）
CREATE INDEX idx_payment_methods_last4 ON payment_methods (user_id, card_last4);
