-- 3.11 categories: 支出カテゴリのマスタ。親子関係を持てる。
CREATE TABLE categories (
    id         bigserial   PRIMARY KEY,
    user_id    bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       varchar(50) NOT NULL,
    parent_id  bigint      REFERENCES categories(id) ON DELETE SET NULL,
    sort_order integer     NOT NULL DEFAULT 0,
    is_system  boolean     NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    -- 論理削除にすると削除済みの行がこの制約を占有し、同名で作り直せなくなるため
    -- categories は物理削除とする（DB 仕様書 3.7 の削除方針）
    CONSTRAINT uq_categories_user_name UNIQUE (user_id, name)
);

-- カテゴリ一覧の表示（API-019）
CREATE INDEX idx_categories_user ON categories (user_id, sort_order);
