package repository_test

import (
	"errors"
	"testing"
	"time"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/model"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/repository"
)

var jst = time.FixedZone("Asia/Tokyo", 9*60*60)

// txnFixture は取引を作るための最小限の指定。
type txnFixture struct {
	occurredAt time.Time
	amount     int64
	categoryID *int64
	merchantID *int64
	note       *string
	duplicate  bool
}

func insertTxn(t *testing.T, repo *repository.Transaction, userID int64, f txnFixture) *model.Transaction {
	t.Helper()

	if f.occurredAt.IsZero() {
		f.occurredAt = time.Date(2026, 9, 10, 12, 0, 0, 0, jst)
	}
	if f.amount == 0 {
		f.amount = 1000
	}

	txn := &model.Transaction{
		UserID:              userID,
		OccurredAt:          f.occurredAt,
		AmountMinor:         f.amount,
		Currency:            model.CurrencyJPY,
		AmountJPYMinor:      f.amount,
		CategoryID:          f.categoryID,
		MerchantID:          f.merchantID,
		CategorySource:      model.CategorySourceDefault,
		Source:              model.TransactionSourceManual,
		Status:              model.TransactionStatusConfirmed,
		IsPossibleDuplicate: f.duplicate,
		Note:                f.note,
	}
	if err := repo.Create(t.Context(), txn); err != nil {
		t.Fatalf("取引の作成に失敗しました: %v", err)
	}
	return txn
}

func TestTransaction_List_新着順と集計(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewTransaction(tx)

	user := newUser(t, userRepo)

	insertTxn(t, repo, user.ID, txnFixture{
		occurredAt: time.Date(2026, 9, 1, 10, 0, 0, 0, jst), amount: 100,
	})
	insertTxn(t, repo, user.ID, txnFixture{
		occurredAt: time.Date(2026, 9, 10, 10, 0, 0, 0, jst), amount: 200,
	})
	insertTxn(t, repo, user.ID, txnFixture{
		occurredAt: time.Date(2026, 9, 5, 10, 0, 0, 0, jst), amount: 300,
	})

	result, err := repo.List(t.Context(), user.ID, repository.ListFilter{Limit: 20})
	if err != nil {
		t.Fatalf("一覧の取得に失敗しました: %v", err)
	}

	if result.Total != 3 {
		t.Errorf("Total: got %d, want 3", result.Total)
	}
	if result.TotalAmountMinor != 600 {
		t.Errorf("TotalAmountMinor: got %d, want 600", result.TotalAmountMinor)
	}

	// occurred_at の降順（新着順）
	for i := 1; i < len(result.Rows); i++ {
		if result.Rows[i-1].OccurredAt.Before(result.Rows[i].OccurredAt) {
			t.Errorf("新着順になっていません: %v の後に %v",
				result.Rows[i-1].OccurredAt, result.Rows[i].OccurredAt)
		}
	}
}

// TestTransaction_List_論理削除は除外される は削除方針の中心を確認する。
//
// 参照系のクエリは常に deleted_at IS NULL で絞る必要がある（DB 仕様書 3.7）。
// 付け忘れると削除済みの取引が画面に出るうえ、合計金額も狂う。
func TestTransaction_List_論理削除は除外される(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewTransaction(tx)

	user := newUser(t, userRepo)

	keep := insertTxn(t, repo, user.ID, txnFixture{amount: 1000})
	removed := insertTxn(t, repo, user.ID, txnFixture{amount: 5000})

	if err := repo.SoftDelete(t.Context(), removed.ID, time.Now()); err != nil {
		t.Fatalf("論理削除に失敗しました: %v", err)
	}

	result, err := repo.List(t.Context(), user.ID, repository.ListFilter{Limit: 20})
	if err != nil {
		t.Fatalf("一覧の取得に失敗しました: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("Total: got %d, want 1", result.Total)
	}
	// 合計金額にも削除済みが混ざらないこと
	if result.TotalAmountMinor != 1000 {
		t.Errorf("TotalAmountMinor: got %d, want 1000", result.TotalAmountMinor)
	}
	if len(result.Rows) != 1 || result.Rows[0].ID != keep.ID {
		t.Errorf("残った取引が想定と異なります: %+v", result.Rows)
	}

	// 詳細取得でも見えないこと
	if _, err := repo.FindByID(t.Context(), removed.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("削除済みを取得できてしまいます: %v", err)
	}
}

func TestTransaction_List_他ユーザの取引を含めない(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewTransaction(tx)

	me := newUser(t, userRepo)

	other := &model.User{
		GoogleSub: uniqueSub(t) + "-other",
		Email:     uniqueSub(t) + "-other@example.com",
		Timezone:  "Asia/Tokyo",
	}
	if err := userRepo.CreateWithInitialData(t.Context(), other); err != nil {
		t.Fatalf("他ユーザの作成に失敗しました: %v", err)
	}

	insertTxn(t, repo, me.ID, txnFixture{amount: 1000})
	insertTxn(t, repo, other.ID, txnFixture{amount: 9999})

	result, err := repo.List(t.Context(), me.ID, repository.ListFilter{Limit: 20})
	if err != nil {
		t.Fatalf("一覧の取得に失敗しました: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("Total: got %d, want 1", result.Total)
	}
	if result.TotalAmountMinor != 1000 {
		t.Errorf("TotalAmountMinor: got %d, want 1000（他ユーザ分が混ざっています）",
			result.TotalAmountMinor)
	}
}

// TestTransaction_List_期間で絞り込む は from / to の境界を確認する。
func TestTransaction_List_期間で絞り込む(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewTransaction(tx)

	user := newUser(t, userRepo)

	for _, day := range []int{1, 5, 10} {
		insertTxn(t, repo, user.ID, txnFixture{
			occurredAt: time.Date(2026, 9, day, 12, 0, 0, 0, jst),
			amount:     100,
		})
	}

	from := time.Date(2026, 9, 5, 0, 0, 0, 0, jst)
	// ハンドラは to の翌日 0 時を渡すため、ここでも同じ形にする
	to := time.Date(2026, 9, 11, 0, 0, 0, 0, jst)

	result, err := repo.List(t.Context(), user.ID, repository.ListFilter{
		From: &from, To: &to, Limit: 20,
	})
	if err != nil {
		t.Fatalf("一覧の取得に失敗しました: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("Total: got %d, want 2（9/5 と 9/10）", result.Total)
	}
}

// TestTransaction_List_ページネーション は limit / offset と meta の関係を確認する。
//
// total は絞り込み条件に合致する全体の件数であり、
// limit で切り取った件数ではないこと。
func TestTransaction_List_ページネーション(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewTransaction(tx)

	user := newUser(t, userRepo)

	for i := range 5 {
		insertTxn(t, repo, user.ID, txnFixture{
			occurredAt: time.Date(2026, 9, i+1, 12, 0, 0, 0, jst),
			amount:     100,
		})
	}

	result, err := repo.List(t.Context(), user.ID, repository.ListFilter{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("一覧の取得に失敗しました: %v", err)
	}

	if len(result.Rows) != 2 {
		t.Errorf("取得件数: got %d, want 2", len(result.Rows))
	}
	if result.Total != 5 {
		t.Errorf("Total: got %d, want 5（limit で切り取る前の件数）", result.Total)
	}
	if result.TotalAmountMinor != 500 {
		t.Errorf("TotalAmountMinor: got %d, want 500（全体の合計）", result.TotalAmountMinor)
	}

	// 2 ページ目
	page2, err := repo.List(t.Context(), user.ID, repository.ListFilter{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("一覧の取得に失敗しました: %v", err)
	}
	if len(page2.Rows) != 2 {
		t.Errorf("2 ページ目の件数: got %d, want 2", len(page2.Rows))
	}
	if page2.Rows[0].ID == result.Rows[0].ID {
		t.Error("offset が効いていません（同じ行が返っています）")
	}
}

// TestTransaction_List_部分一致検索 は q による店舗名・メモの検索を確認する。
func TestTransaction_List_部分一致検索(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewTransaction(tx)
	merchantRepo := repository.NewMerchant(tx)

	user := newUser(t, userRepo)

	merchant, err := merchantRepo.FindOrCreateByName(t.Context(), user.ID, "セブン-イレブン渋谷店")
	if err != nil {
		t.Fatalf("店舗の作成に失敗しました: %v", err)
	}

	note := "ランチ代"
	insertTxn(t, repo, user.ID, txnFixture{merchantID: &merchant.ID})
	insertTxn(t, repo, user.ID, txnFixture{note: &note})
	insertTxn(t, repo, user.ID, txnFixture{})

	tests := []struct {
		name  string
		query string
		want  int64
	}{
		{name: "店舗名で引っかかる", query: "セブン", want: 1},
		{name: "メモで引っかかる", query: "ランチ", want: 1},
		{name: "該当なし", query: "存在しない文字列", want: 0},
		// LIKE のワイルドカードがエスケープされ、全件一致にならないこと
		{name: "% はワイルドカードとして扱わない", query: "%", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := tt.query
			result, lerr := repo.List(t.Context(), user.ID, repository.ListFilter{
				Query: &q, Limit: 20,
			})
			if lerr != nil {
				t.Fatalf("一覧の取得に失敗しました: %v", lerr)
			}
			if result.Total != tt.want {
				t.Errorf("Total: got %d, want %d", result.Total, tt.want)
			}
		})
	}
}

// TestTransaction_List_結合したマスタ名を返す は JOIN の結果を確認する。
//
// GORM は読み込み先の構造体から列名を推測するため、結合時は Select の明示が必要。
func TestTransaction_List_結合したマスタ名を返す(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewTransaction(tx)
	merchantRepo := repository.NewMerchant(tx)
	categoryRepo := repository.NewCategory(tx)

	user := newUser(t, userRepo)

	merchant, err := merchantRepo.FindOrCreateByName(t.Context(), user.ID, "近所の定食屋")
	if err != nil {
		t.Fatalf("店舗の作成に失敗しました: %v", err)
	}

	categories, err := categoryRepo.List(t.Context(), user.ID, false)
	if err != nil {
		t.Fatalf("カテゴリ一覧の取得に失敗しました: %v", err)
	}
	category := categories[0]

	insertTxn(t, repo, user.ID, txnFixture{
		merchantID: &merchant.ID,
		categoryID: &category.ID,
	})

	result, err := repo.List(t.Context(), user.ID, repository.ListFilter{Limit: 20})
	if err != nil {
		t.Fatalf("一覧の取得に失敗しました: %v", err)
	}

	row := result.Rows[0]
	if row.MerchantName == nil || *row.MerchantName != "近所の定食屋" {
		t.Errorf("MerchantName: got %v, want 近所の定食屋", row.MerchantName)
	}
	if row.CategoryName == nil || *row.CategoryName != category.Name {
		t.Errorf("CategoryName: got %v, want %q", row.CategoryName, category.Name)
	}
	// マスタが紐づいていない場合は NULL のままであること
	if row.PaymentMethodName != nil {
		t.Errorf("PaymentMethodName: got %v, want nil", *row.PaymentMethodName)
	}
}

func TestTransaction_Update(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewTransaction(tx)

	user := newUser(t, userRepo)
	txn := insertTxn(t, repo, user.ID, txnFixture{amount: 1000})

	err := repo.Update(t.Context(), txn.ID, map[string]any{
		"amount_minor":     2500,
		"amount_jpy_minor": 2500,
		"is_user_edited":   true,
	})
	if err != nil {
		t.Fatalf("更新に失敗しました: %v", err)
	}

	found, err := repo.FindByID(t.Context(), txn.ID)
	if err != nil {
		t.Fatalf("取得に失敗しました: %v", err)
	}
	if found.AmountMinor != 2500 {
		t.Errorf("AmountMinor: got %d, want 2500", found.AmountMinor)
	}
	if !found.IsUserEdited {
		t.Error("IsUserEdited が false です（再計算の対象外にならない）")
	}
}

// TestTransaction_SoftDelete_行は残る は論理削除の実体を確認する。
//
// 物理削除すると決済イベントが未名寄せに戻り、次のバッチで
// 同じ取引が再生成されてしまう（DB 仕様書 3.7 の理由 1）。
func TestTransaction_SoftDelete_行は残る(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewTransaction(tx)

	user := newUser(t, userRepo)
	txn := insertTxn(t, repo, user.ID, txnFixture{})

	if err := repo.SoftDelete(t.Context(), txn.ID, time.Now()); err != nil {
		t.Fatalf("論理削除に失敗しました: %v", err)
	}

	// 行そのものは残っていること
	var count int64
	err := tx.Raw("SELECT COUNT(*) FROM transactions WHERE id = ?", txn.ID).Scan(&count).Error
	if err != nil {
		t.Fatalf("件数の取得に失敗しました: %v", err)
	}
	if count != 1 {
		t.Errorf("行が物理削除されています: count=%d", count)
	}

	var deletedAt *time.Time
	err = tx.Raw("SELECT deleted_at FROM transactions WHERE id = ?", txn.ID).Scan(&deletedAt).Error
	if err != nil {
		t.Fatalf("deleted_at の取得に失敗しました: %v", err)
	}
	if deletedAt == nil {
		t.Error("deleted_at が設定されていません")
	}
}

func TestMerchant_FindOrCreateByName(t *testing.T) {
	tx := beginTx(t)
	userRepo := repository.NewUser(tx)
	repo := repository.NewMerchant(tx)

	user := newUser(t, userRepo)

	first, err := repo.FindOrCreateByName(t.Context(), user.ID, "近所の定食屋")
	if err != nil {
		t.Fatalf("作成に失敗しました: %v", err)
	}

	// 同じ名前で呼んでも店舗が増えないこと
	second, err := repo.FindOrCreateByName(t.Context(), user.ID, "近所の定食屋")
	if err != nil {
		t.Fatalf("取得に失敗しました: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("同じ名前で別の店舗が作られました: %d と %d", first.ID, second.ID)
	}

	var count int64
	cerr := tx.Model(&model.Merchant{}).
		Where("user_id = ? AND name = ?", user.ID, "近所の定食屋").
		Count(&count).Error
	if cerr != nil {
		t.Fatalf("件数の取得に失敗しました: %v", cerr)
	}
	if count != 1 {
		t.Errorf("店舗数: got %d, want 1", count)
	}
}
