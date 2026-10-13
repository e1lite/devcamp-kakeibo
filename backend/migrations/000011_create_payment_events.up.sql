-- 3.6 payment_events: メール 1 通から抽出した決済イベント（名寄せ前）。
-- 1 通のメールから複数行が生成されうる（銀行の入出金まとめ通知など）。
--
-- 最小版（docs/mail-import-mvp.md）では、利用通知 1 通 = イベント 1 件 = 取引 1 件とし、
-- 名寄せ（P5）はしない。link_type は常に auto、link_score は使わない。
CREATE TABLE payment_events (
    id                 bigserial    PRIMARY KEY,
    user_id            bigint       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email_message_id   bigint       NOT NULL REFERENCES email_messages(id) ON DELETE CASCADE,
    parser_template_id bigint       REFERENCES parser_templates(id) ON DELETE SET NULL,
    -- 名寄せの結果。未処理の間は NULL
    transaction_id     bigint       REFERENCES transactions(id) ON DELETE SET NULL,
    link_type          varchar(20),
    link_score         numeric(5,2),
    linked_at          timestamptz,
    event_type         varchar(20)  NOT NULL,
    occurred_at        timestamptz  NOT NULL,
    amount_minor       bigint       NOT NULL,
    currency           char(3)      NOT NULL DEFAULT 'JPY',
    amount_jpy_minor   bigint       NOT NULL,
    raw_merchant_text  varchar(255),
    merchant_id        bigint       REFERENCES merchants(id) ON DELETE SET NULL,
    payment_method_id  bigint       REFERENCES payment_methods(id) ON DELETE SET NULL,
    card_last4         varchar(4),
    normalized_at      timestamptz,
    -- 即時通知の送信済み日時。通知の冪等性を担保する（最小版では使わない）
    notified_at        timestamptz,
    created_at         timestamptz  NOT NULL DEFAULT now(),
    updated_at         timestamptz  NOT NULL DEFAULT now(),

    CONSTRAINT ck_payment_events_event_type
        CHECK (event_type IN ('order_confirm', 'usage_notice', 'settlement', 'receipt', 'bank_debit')),
    CONSTRAINT ck_payment_events_link_type
        CHECK (link_type IS NULL OR link_type IN ('auto', 'manual'))
);

-- 一覧・取引詳細での参照
CREATE INDEX idx_payment_events_user_occurred ON payment_events (user_id, occurred_at DESC);

-- 名寄せ（P5）の候補生成。金額一致 + 日時近接で絞る
CREATE INDEX idx_payment_events_reconcile ON payment_events (user_id, amount_jpy_minor, occurred_at);

-- 取引に紐づくイベントの取得
CREATE INDEX idx_payment_events_transaction ON payment_events (transaction_id);

-- 未通知イベントの抽出
CREATE INDEX idx_payment_events_unnotified ON payment_events (user_id, notified_at)
    WHERE notified_at IS NULL;

-- メールからのイベントの取得
CREATE INDEX idx_payment_events_email ON payment_events (email_message_id);
