-- oauth_states: OAuth の state（ログイン・メール連携の CSRF 対策）。
--
-- これまでは API サーバのメモリに置いていたが、本番では API サーバを複数台で動かすため、
-- 認可の開始（API-002 / API-029）とコールバック（API-003 / API-030）が別の台に振り分けられると、
-- 受けた台に state が無く、400 INVALID_STATE で失敗する。台をまたいで共有できるよう DB に置く
-- （docs/step7-infra.md 6 章 #1・7.4）。
--
-- 10 分で失効し、1 度使ったら削除する。発行のたびに期限切れの行を消すため、行は溜まらない。
CREATE TABLE oauth_states (
    state        varchar(64)  PRIMARY KEY,
    purpose      varchar(20)  NOT NULL,
    redirect_uri text         NOT NULL,
    -- メール連携を始めたユーザ。ログインでは NULL
    user_id      bigint       REFERENCES users(id) ON DELETE CASCADE,
    expires_at   timestamptz  NOT NULL,
    created_at   timestamptz  NOT NULL DEFAULT now(),

    CONSTRAINT ck_oauth_states_purpose
        CHECK (purpose IN ('login', 'mail_link'))
);

-- 期限切れの行をまとめて消すため
CREATE INDEX idx_oauth_states_expires_at ON oauth_states (expires_at);
