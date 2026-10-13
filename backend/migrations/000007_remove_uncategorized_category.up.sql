-- 「未分類」の表現を category_id = NULL に一本化する（API 仕様書 API-003、Step 4 で変更）。
--
-- これまでは「未分類」が 2 通りに分かれていた。
--   1. 初期カテゴリとして作った「未分類」（is_system = true）を指す取引
--   2. カテゴリを選ばなかった取引・カテゴリを削除した取引（category_id = NULL）
-- 画面で「未分類」を絞り込むと片方しか出てこないため、2 に揃える。
--
-- 初期カテゴリの「未分類」を削除すると、指していた取引は FK の
-- ON DELETE SET NULL により category_id = NULL（= 未分類）になる。
-- 子カテゴリの parent_id も同じく NULL になり、最上位に繰り上がる。
--
-- is_system = true に限るのは、利用者が自分で作った同名のカテゴリを消さないため
-- （初期カテゴリの「未分類」を別名に変えたうえで「未分類」を作り直した場合など）。
DELETE FROM categories
WHERE name = '未分類'
  AND is_system = true;
