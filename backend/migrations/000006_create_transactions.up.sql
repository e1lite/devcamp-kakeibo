-- 3.7 transactions: 名寄せ後の実取引。1 回の支出 = 1 行。
--
-- payment_events を参照せず自前のフィールドを持つ。
-- 名寄せロジックを変更して全件再計算しても、ユーザの手修正が失われないようにするため。
--
-- このテーブルのみ論理削除（deleted_at）とする。
-- 物理削除すると決済イベントが未名寄せに戻り、次のバッチで同じ取引が再生成されるため。
CREATE TABLE transactions (
    id                    bigserial   PRIMARY KEY,
    user_id               bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    occurred_at           timestamptz NOT NULL,
    amount_minor          bigint      NOT NULL,
    currency              char(3)     NOT NULL DEFAULT 'JPY',
    -- 集計はこの列を使う（通貨が混在しても合算できるようにするため）
    amount_jpy_minor      bigint      NOT NULL,
    merchant_id           bigint      REFERENCES merchants(id) ON DELETE SET NULL,
    category_id           bigint      REFERENCES categories(id) ON DELETE SET NULL,
    payment_method_id     bigint      REFERENCES payment_methods(id) ON DELETE SET NULL,
    -- ユーザ修正をルールで上書きしないための出所記録
    category_source       varchar(20) NOT NULL DEFAULT 'default',
    source                varchar(20) NOT NULL,
    status                varchar(20) NOT NULL DEFAULT 'confirmed',
    -- true の行は再計算の対象外にする
    is_user_edited        boolean     NOT NULL DEFAULT false,
    is_possible_duplicate boolean     NOT NULL DEFAULT false,
    -- マージで統合された先の取引 ID（自己参照）。NULL 以外はマージ済みを意味する
    merged_into_id        bigint      REFERENCES transactions(id) ON DELETE SET NULL,
    note                  text,
    -- 論理削除の実施日時。参照時は常に deleted_at IS NULL で絞る
    deleted_at            timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ck_transactions_amount_positive CHECK (amount_minor > 0),
    CONSTRAINT ck_transactions_category_source
        CHECK (category_source IN ('rule', 'place_type', 'user', 'default')),
    CONSTRAINT ck_transactions_source
        CHECK (source IN ('email', 'manual')),
    CONSTRAINT ck_transactions_status
        CHECK (status IN ('pending', 'confirmed'))
);

-- 参照系のクエリは常に deleted_at IS NULL で絞るため、主要インデックスは
-- この条件を付けた部分インデックスにする。削除済みの行が索引に載らず、
-- 通常の一覧・集計が削除件数に影響されない（DB 仕様書 3.7）。

-- 取引一覧の主軸（期間絞り込み・新着順）: API-010
CREATE INDEX idx_transactions_user_occurred
    ON transactions (user_id, occurred_at DESC)
    WHERE deleted_at IS NULL;

-- 月次サマリのカテゴリ別集計: API-023
CREATE INDEX idx_transactions_user_category_occurred
    ON transactions (user_id, category_id, occurred_at)
    WHERE deleted_at IS NULL;

-- 店舗別集計・サブスク検出
CREATE INDEX idx_transactions_merchant ON transactions (merchant_id);

-- 要確認キューの取得
CREATE INDEX idx_transactions_review
    ON transactions (user_id, is_possible_duplicate)
    WHERE is_possible_duplicate AND deleted_at IS NULL;

-- マージ解除時に統合元を引く: API-016
CREATE INDEX idx_transactions_merged
    ON transactions (merged_into_id)
    WHERE merged_into_id IS NOT NULL;
