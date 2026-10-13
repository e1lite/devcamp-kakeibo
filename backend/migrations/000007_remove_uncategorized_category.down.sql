-- 初期カテゴリの「未分類」を作り直す。
--
-- up で category_id = NULL になった取引は、元々「未分類」を指していたのか、
-- もともとカテゴリが無かったのかを区別できないため、取引の紐づけは戻さない。
-- 同名のカテゴリがすでにあるユーザには作らない（UNIQUE(user_id, name)）。
INSERT INTO categories (user_id, name, sort_order, is_system)
SELECT id, '未分類', 9, true
FROM users
ON CONFLICT (user_id, name) DO NOTHING;
