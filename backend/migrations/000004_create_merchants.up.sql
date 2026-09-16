-- 3.8 merchants: 正規化済みの店舗マスタ。1 行 = 1 支店とし、brand_name でチェーンを束ねる。
CREATE TABLE merchants (
    id             bigserial    PRIMARY KEY,
    user_id        bigint       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name           varchar(255) NOT NULL,
    brand_name     varchar(100),
    is_online      boolean      NOT NULL DEFAULT false,
    address        varchar(255),
    latitude       numeric(9,6),
    longitude      numeric(9,6),
    place_id       varchar(255),
    place_types    varchar(255),
    geocode_status varchar(20)  NOT NULL DEFAULT 'pending',
    geocoded_at    timestamptz,
    created_at     timestamptz  NOT NULL DEFAULT now(),
    updated_at     timestamptz  NOT NULL DEFAULT now(),

    CONSTRAINT uq_merchants_user_name UNIQUE (user_id, name),
    CONSTRAINT ck_merchants_geocode_status
        CHECK (geocode_status IN ('pending', 'success', 'not_found', 'skipped'))
);

-- ブランド単位の集計・カテゴリ規則の適用
CREATE INDEX idx_merchants_user_brand ON merchants (user_id, brand_name);

-- 未ジオコーディング店舗の抽出。該当行が少ないため部分インデックスにする
CREATE INDEX idx_merchants_geocode_queue ON merchants (geocode_status)
    WHERE geocode_status = 'pending';
