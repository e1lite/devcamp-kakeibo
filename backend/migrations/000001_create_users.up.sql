-- 3.1 users: アプリの利用者。認証は Google OAuth に委譲するためパスワードは保持しない。
CREATE TABLE users (
    id           bigserial    PRIMARY KEY,
    google_sub   varchar(255) NOT NULL UNIQUE,
    email        varchar(255) NOT NULL UNIQUE,
    display_name varchar(100),
    timezone     varchar(50)  NOT NULL DEFAULT 'Asia/Tokyo',
    created_at   timestamptz  NOT NULL DEFAULT now(),
    updated_at   timestamptz  NOT NULL DEFAULT now()
);

-- 追加のインデックスは定義しない。
-- ログイン時の本人特定に使う google_sub の検索は、UNIQUE 制約に対して
-- PostgreSQL が自動生成する一意インデックスで賄える（DB 仕様書 3.1）。
