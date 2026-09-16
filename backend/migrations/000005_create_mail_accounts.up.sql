-- 3.2 mail_accounts: 連携したメールアカウントと認証情報。
-- 取込方式を provider で切り替えられるようにし、Gmail API / GAS / IMAP を同じテーブルで扱う。
--
-- Step 3 ではログイン時のアカウント情報保持までを実装し、
-- メール取得の実行は Step 8 以降に回す（API 仕様書 5.1）。
CREATE TABLE mail_accounts (
    id                    bigserial    PRIMARY KEY,
    user_id               bigint       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email_address         varchar(255) NOT NULL,
    provider              varchar(20)  NOT NULL,
    -- 連携解除時は空文字に更新する（行は削除しない。DB 仕様書 3.2 の削除方針）
    credential_encrypted  text         NOT NULL DEFAULT '',
    credential_expires_at timestamptz,
    sync_cursor           varchar(255),
    backfilled_until      timestamptz,
    -- 同期が成功したら 0 件でも更新する。死活監視はこの列で行う（DB 仕様書 3.3）
    last_synced_at        timestamptz,
    status                varchar(20)  NOT NULL DEFAULT 'active',
    created_at            timestamptz  NOT NULL DEFAULT now(),
    updated_at            timestamptz  NOT NULL DEFAULT now(),

    CONSTRAINT uq_mail_accounts_user_email UNIQUE (user_id, email_address),
    CONSTRAINT ck_mail_accounts_provider
        CHECK (provider IN ('gmail_api', 'gas', 'imap')),
    CONSTRAINT ck_mail_accounts_status
        CHECK (status IN ('active', 'reauth_required', 'disabled'))
);

-- バッチが同期対象アカウントを抽出する
CREATE INDEX idx_mail_accounts_status ON mail_accounts (status);

-- ユーザの連携一覧取得（WHERE user_id = ?）は uq_mail_accounts_user_email の
-- 左端プレフィックスで賄えるため、専用インデックスは定義しない（DB 仕様書 3.2）。
