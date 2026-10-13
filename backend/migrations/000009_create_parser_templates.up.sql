-- 3.5 parser_templates: 送信元・件名から決済メールかどうかを判定し、本文から項目を抽出するルール。
--
-- email_messages.matched_template_id から参照されるため、email_messages より先に作る。
-- 最小版では全ユーザ共通（user_id = NULL）のテンプレートを、実際のメールを見てから投入する。
CREATE TABLE parser_templates (
    id                bigserial    PRIMARY KEY,
    user_id           bigint       REFERENCES users(id) ON DELETE CASCADE,
    name              varchar(100) NOT NULL,
    from_pattern      varchar(255) NOT NULL,
    subject_pattern   varchar(255),
    event_type        varchar(20)  NOT NULL,
    payment_method_id bigint       REFERENCES payment_methods(id) ON DELETE SET NULL,
    body_format       varchar(10)  NOT NULL DEFAULT 'text',
    extraction_rules  jsonb        NOT NULL,
    is_multi_event    boolean      NOT NULL DEFAULT false,
    priority          integer      NOT NULL DEFAULT 100,
    is_active         boolean      NOT NULL DEFAULT true,
    created_by        varchar(20)  NOT NULL DEFAULT 'user',
    created_at        timestamptz  NOT NULL DEFAULT now(),
    updated_at        timestamptz  NOT NULL DEFAULT now(),

    CONSTRAINT uq_parser_templates_user_name UNIQUE (user_id, name),
    CONSTRAINT ck_parser_templates_event_type
        CHECK (event_type IN ('order_confirm', 'usage_notice', 'settlement', 'receipt', 'bank_debit')),
    CONSTRAINT ck_parser_templates_body_format
        CHECK (body_format IN ('text', 'html')),
    CONSTRAINT ck_parser_templates_created_by
        CHECK (created_by IN ('system', 'user', 'llm'))
);

-- 判定時の評価順の取得
CREATE INDEX idx_parser_templates_match ON parser_templates (is_active, priority);

-- 送信元からのテンプレート検索
CREATE INDEX idx_parser_templates_from ON parser_templates (from_pattern);
