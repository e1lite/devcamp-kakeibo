-- 3.3 sync_jobs: メール取得ジョブの実行履歴。可観測性（無音の故障を検知する）のために持つ。
--
-- メール取込の最小版（docs/mail-import-mvp.md）では、手動の同期（manual）と
-- 過去メールの遡り取得（backfill）だけを使う。定期実行（scheduled）と Push は拡張で使う。
CREATE TABLE sync_jobs (
    id              bigserial    PRIMARY KEY,
    mail_account_id bigint       NOT NULL REFERENCES mail_accounts(id) ON DELETE CASCADE,
    trigger         varchar(20)  NOT NULL,
    started_at      timestamptz  NOT NULL,
    finished_at     timestamptz,
    status          varchar(20)  NOT NULL,
    fetched_count   integer      NOT NULL DEFAULT 0,
    parsed_count    integer      NOT NULL DEFAULT 0,
    failed_count    integer      NOT NULL DEFAULT 0,
    cursor_before   varchar(255),
    cursor_after    varchar(255),
    error_message   text,
    created_at      timestamptz  NOT NULL DEFAULT now(),

    CONSTRAINT ck_sync_jobs_trigger
        CHECK (trigger IN ('scheduled', 'manual', 'backfill', 'push')),
    CONSTRAINT ck_sync_jobs_status
        CHECK (status IN ('running', 'success', 'partial', 'failed'))
);

-- 同期履歴の表示と、連携ごとの実行中ジョブの確認
CREATE INDEX idx_sync_jobs_account_started ON sync_jobs (mail_account_id, started_at DESC);
