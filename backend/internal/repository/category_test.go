package repository_test

import (
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/model"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/repository"
)

// newCategory はテスト用のカテゴリを作って返す。
func newCategory(t *testing.T, repo *repository.Category, userID int64, name string, parentID *int64) *model.Category {
	t.Helper()

	category := &model.Category{
		UserID:   userID,
		Name:     name,
		ParentID: parentID,
	}
	if err := repo.Create(t.Context(), category); err != nil {
		t.Fatalf("カテゴリの作成に失敗しました: %v", err)
	}
	return category
}

func TestCategory_List(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewCategory(tx)

	user := newUser(t, userRepo)

	// ログイン時の初期カテゴリ 9 件が sort_order 1〜9 で入っている
	categories, err := repo.List(t.Context(), user.ID, false)
	if err != nil {
		t.Fatalf("一覧の取得に失敗しました: %v", err)
	}
	if len(categories) != 9 {
		t.Fatalf("件数: got %d, want 9", len(categories))
	}

	// sort_order の昇順であること
	for i := 1; i < len(categories); i++ {
		if categories[i-1].SortOrder > categories[i].SortOrder {
			t.Errorf("並び順が sort_order 昇順になっていません: %d の後に %d",
				categories[i-1].SortOrder, categories[i].SortOrder)
		}
	}
	if categories[0].Name != "食費" {
		t.Errorf("先頭: got %q, want %q", categories[0].Name, "食費")
	}
}

// TestCategory_List_他ユーザのカテゴリを含めない はマルチテナントの分離を確認する。
func TestCategory_List_他ユーザのカテゴリを含めない(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewCategory(tx)

	me := newUser(t, userRepo)

	other := &model.User{
		GoogleSub: uniqueSub(t) + "-other",
		Email:     uniqueSub(t) + "-other@example.com",
		Timezone:  "Asia/Tokyo",
	}
	if err := userRepo.CreateWithInitialData(t.Context(), other); err != nil {
		t.Fatalf("他ユーザの作成に失敗しました: %v", err)
	}
	newCategory(t, repo, other.ID, "他人のカテゴリ", nil)

	categories, err := repo.List(t.Context(), me.ID, false)
	if err != nil {
		t.Fatalf("一覧の取得に失敗しました: %v", err)
	}

	for _, c := range categories {
		if c.UserID != me.ID {
			t.Errorf("他ユーザのカテゴリが含まれています: id=%d user_id=%d", c.ID, c.UserID)
		}
	}
	if len(categories) != 9 {
		t.Errorf("件数: got %d, want 9", len(categories))
	}
}

// TestCategory_List_取引件数 は include_counts の集計を確認する。
func TestCategory_List_取引件数(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewCategory(tx)

	user := newUser(t, userRepo)

	categories, err := repo.List(t.Context(), user.ID, false)
	if err != nil {
		t.Fatalf("一覧の取得に失敗しました: %v", err)
	}
	target := categories[0]

	// 有効な取引 2 件と、論理削除済み 1 件を作る
	createTransaction(t, tx, user.ID, target.ID, false)
	createTransaction(t, tx, user.ID, target.ID, false)
	createTransaction(t, tx, user.ID, target.ID, true)

	withCounts, err := repo.List(t.Context(), user.ID, true)
	if err != nil {
		t.Fatalf("一覧の取得に失敗しました: %v", err)
	}

	for _, c := range withCounts {
		want := 0
		if c.ID == target.ID {
			// 論理削除済みは数えない（DB 仕様書 3.7）
			want = 2
		}
		if c.TransactionCount != want {
			t.Errorf("%s の取引件数: got %d, want %d", c.Name, c.TransactionCount, want)
		}
	}
}

func TestCategory_Create_重複名はErrDuplicateNameを返す(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewCategory(tx)

	user := newUser(t, userRepo)

	newCategory(t, repo, user.ID, "交際費", nil)

	err := repo.Create(t.Context(), &model.Category{UserID: user.ID, Name: "交際費"})
	if !errors.Is(err, repository.ErrDuplicateName) {
		t.Errorf("got %v, want ErrDuplicateName", err)
	}
}

// TestCategory_Create_他ユーザとは同名でも作れる は UNIQUE(user_id, name) の範囲を確認する。
func TestCategory_Create_他ユーザとは同名でも作れる(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewCategory(tx)

	me := newUser(t, userRepo)

	other := &model.User{
		GoogleSub: uniqueSub(t) + "-other",
		Email:     uniqueSub(t) + "-other@example.com",
		Timezone:  "Asia/Tokyo",
	}
	if err := userRepo.CreateWithInitialData(t.Context(), other); err != nil {
		t.Fatalf("他ユーザの作成に失敗しました: %v", err)
	}

	newCategory(t, repo, me.ID, "交際費", nil)

	// 別ユーザなら同じ名前でも作れる
	err := repo.Create(t.Context(), &model.Category{UserID: other.ID, Name: "交際費"})
	if err != nil {
		t.Errorf("他ユーザの同名カテゴリを作成できません: %v", err)
	}
}

// TestCategory_Delete_取引のカテゴリはNULLになる は FK の SET NULL を確認する。
//
// 物理削除しても取引そのものは消えないこと、参照が NULL になることを保証する。
func TestCategory_Delete_取引のカテゴリはNULLになる(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewCategory(tx)

	user := newUser(t, userRepo)
	category := newCategory(t, repo, user.ID, "消す予定", nil)
	txnID := createTransaction(t, tx, user.ID, category.ID, false)

	if err := repo.Delete(t.Context(), category.ID); err != nil {
		t.Fatalf("削除に失敗しました: %v", err)
	}

	var categoryID *int64
	err := tx.Raw("SELECT category_id FROM transactions WHERE id = ?", txnID).Scan(&categoryID).Error
	if err != nil {
		t.Fatalf("取引の取得に失敗しました: %v", err)
	}
	if categoryID != nil {
		t.Errorf("category_id: got %v, want nil（SET NULL が効いていません）", *categoryID)
	}
}

// TestCategory_Delete_子カテゴリは親を外される は parent_id の SET NULL を確認する。
// ツリーが壊れて孤児が残らないこと。
func TestCategory_Delete_子カテゴリは親を外される(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewCategory(tx)

	user := newUser(t, userRepo)
	parent := newCategory(t, repo, user.ID, "親", nil)
	child := newCategory(t, repo, user.ID, "子", &parent.ID)

	if err := repo.Delete(t.Context(), parent.ID); err != nil {
		t.Fatalf("削除に失敗しました: %v", err)
	}

	found, err := repo.FindByID(t.Context(), child.ID)
	if err != nil {
		t.Fatalf("子カテゴリが消えています: %v", err)
	}
	if found.ParentID != nil {
		t.Errorf("parent_id: got %v, want nil", *found.ParentID)
	}
}

func TestCategory_ExistsForUser(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewCategory(tx)

	me := newUser(t, userRepo)
	mine := newCategory(t, repo, me.ID, "自分の", nil)

	other := &model.User{
		GoogleSub: uniqueSub(t) + "-other",
		Email:     uniqueSub(t) + "-other@example.com",
		Timezone:  "Asia/Tokyo",
	}
	if err := userRepo.CreateWithInitialData(t.Context(), other); err != nil {
		t.Fatalf("他ユーザの作成に失敗しました: %v", err)
	}
	theirs := newCategory(t, repo, other.ID, "他人の", nil)

	tests := []struct {
		name string
		id   int64
		want bool
	}{
		{name: "自分のカテゴリ", id: mine.ID, want: true},
		// FK 制約だけでは他ユーザのカテゴリを親にできてしまうため、
		// user_id まで含めて確認する必要がある
		{name: "他ユーザのカテゴリ", id: theirs.ID, want: false},
		{name: "存在しない ID", id: 999999, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.ExistsForUser(t.Context(), tt.id, me.ID)
			if err != nil {
				t.Fatalf("確認に失敗しました: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCategory_IsDescendantOrSelf は親の付け替えで循環が生まれないかの判定を確認する。
//
//	祖父 → 親 → 子 という階層を作り、それぞれの関係を検証する。
func TestCategory_IsDescendantOrSelf(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewCategory(tx)

	user := newUser(t, userRepo)
	grandparent := newCategory(t, repo, user.ID, "祖父", nil)
	parent := newCategory(t, repo, user.ID, "親", &grandparent.ID)
	child := newCategory(t, repo, user.ID, "子", &parent.ID)
	unrelated := newCategory(t, repo, user.ID, "無関係", nil)

	tests := []struct {
		name      string
		candidate int64
		target    int64
		want      bool
	}{
		{name: "自分自身", candidate: parent.ID, target: parent.ID, want: true},
		{name: "直接の子", candidate: child.ID, target: parent.ID, want: true},
		{name: "孫", candidate: child.ID, target: grandparent.ID, want: true},
		{name: "親は子孫ではない", candidate: grandparent.ID, target: child.ID, want: false},
		{name: "無関係", candidate: unrelated.ID, target: parent.ID, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.IsDescendantOrSelf(t.Context(), tt.candidate, tt.target, user.ID)
			if err != nil {
				t.Fatalf("判定に失敗しました: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// createTransaction はテスト用の取引を作り、その ID を返す。
func createTransaction(t *testing.T, tx *gorm.DB, userID, categoryID int64, deleted bool) int64 {
	t.Helper()

	const query = `
		INSERT INTO transactions
			(user_id, occurred_at, amount_minor, currency, amount_jpy_minor,
			 category_id, source, status, deleted_at)
		VALUES (?, now(), 1000, 'JPY', 1000, ?, 'manual', 'confirmed', ?)
		RETURNING id`

	var deletedAt any
	if deleted {
		deletedAt = gorm.Expr("now()")
	}

	var id int64
	if err := tx.Raw(query, userID, categoryID, deletedAt).Scan(&id).Error; err != nil {
		t.Fatalf("取引の作成に失敗しました: %v", err)
	}
	return id
}
