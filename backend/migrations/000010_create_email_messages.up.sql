-- 3.4 email_messages: 取得したメールの生データ。通常の運用では削除しない。
-- 後段（判定・抽出・正規化・名寄せ）のロジックを変えたとき、全件を再計算する起点になる。
-- メール連携を解除しても削除されない（mail_accounts は行を残す。DB 仕様書 3.2）。
CREATE TABLE email_messages (
    id                  bigserial    PRIMARY KEY,
    user_id             bigint       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mail_account_id     bigint       NOT NULL REFERENCES mail_accounts(id) ON DELETE CASCADE,
    provider_message_id varchar(255) NOT NULL,
    thread_id           varchar(255),
    from_address        varchar(255) NOT NULL,
    from_name           varchar(255),
    subject             text,
    received_at         timestamptz  NOT NULL,
    body_text           text,
    body_html           text,
    classification      varchar(20)  NOT NULL DEFAULT 'unknown',
    classified_at       timestamptz,
    matched_template_id bigint       REFERENCES parser_templates(id) ON DELETE SET NULL,
    parse_status        varchar(20)  NOT NULL DEFAULT 'pending',
    parse_error         text,
    created_at          timestamptz  NOT NULL DEFAULT now(),
    updated_at          timestamptz  NOT NULL DEFAULT now(),

    -- 取込の冪等性を担保する中心的な制約。同じメールを再取得しても重複行が増えない
    CONSTRAINT uq_email_messages_account_message UNIQUE (mail_account_id, provider_message_id),
    CONSTRAINT ck_email_messages_classification
        CHECK (classification IN ('payment', 'not_payment', 'unknown')),
    CONSTRAINT ck_email_messages_parse_status
        CHECK (parse_status IN ('pending', 'success', 'failed', 'skipped'))
);

-- 受信一覧の表示
CREATE INDEX idx_email_messages_user_received ON email_messages (user_id, received_at DESC);

-- 未判定・未処理メールの抽出（判定・抽出のキュー）
CREATE INDEX idx_email_messages_queue ON email_messages (user_id, classification, parse_status);

-- 送信元ごとの受信件数の集計（途絶の検知）
CREATE INDEX idx_email_messages_from ON email_messages (from_address);
