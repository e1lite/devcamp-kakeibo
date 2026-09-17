package repository_test

import (
	"errors"
	"fmt"
	"testing"

	"gorm.io/gorm"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/model"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/repository"
)

// uniqueSub はテストごとに衝突しない google_sub を作る。
// 同じデータベースを共有するため、固定値だと UNIQUE 制約でぶつかる。
func uniqueSub(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("test-sub-%s-%d", t.Name(), len(t.Name()))
}

// newUser はテスト用のユーザを作って返す。
func newUser(t *testing.T, repo *repository.User) *model.User {
	t.Helper()

	name := "テスト太郎"
	user := &model.User{
		GoogleSub:   uniqueSub(t),
		Email:       uniqueSub(t) + "@example.com",
		DisplayName: &name,
		Timezone:    "Asia/Tokyo",
	}
	if err := repo.CreateWithInitialData(t.Context(), user); err != nil {
		t.Fatalf("ユーザの作成に失敗しました: %v", err)
	}
	return user
}

func TestUser_CreateWithInitialData(t *testing.T) {
	tx := beginTx(t)
	repo := repository.NewUser(tx)

	user := newUser(t, repo)

	if user.ID == 0 {
		t.Fatal("ID が採番されていません")
	}

	// カテゴリが 1 件もないと取引を登録できないため、
	// ユーザ作成と同時に初期カテゴリが作られること（API 仕様書 API-003）
	var categoryCount int64
	if err := tx.Model(&model.Category{}).
		Where("user_id = ?", user.ID).Count(&categoryCount).Error; err != nil {
		t.Fatalf("カテゴリ数の取得に失敗しました: %v", err)
	}
	if categoryCount != 9 {
		t.Errorf("初期カテゴリ数: got %d, want 9", categoryCount)
	}

	// 「未分類」は自動分類の受け皿になるため、削除できない扱いにしている
	var uncategorized model.Category
	err := tx.Where("user_id = ? AND name = ?", user.ID, "未分類").First(&uncategorized).Error
	if err != nil {
		t.Fatalf("「未分類」カテゴリが見つかりません: %v", err)
	}
	if !uncategorized.IsSystem {
		t.Error("「未分類」の is_system が false です。削除できてしまいます")
	}

	var cash model.PaymentMethod
	if err := tx.Where("user_id = ?", user.ID).First(&cash).Error; err != nil {
		t.Fatalf("初期決済手段が見つかりません: %v", err)
	}
	if cash.Name != "現金" || cash.Kind != "cash" {
		t.Errorf("初期決済手段: got name=%q kind=%q, want 現金/cash", cash.Name, cash.Kind)
	}
}

// TestUser_CreateWithInitialData_失敗時にデータが残らない は、
// ユーザ作成が失敗したときに中途半端なデータが残らないことを確認する。
//
// 件数は開発用データベースの既存データに影響されるため、
// 絶対値ではなく処理の前後の差分で判定する。
func TestUser_CreateWithInitialData_失敗時にデータが残らない(t *testing.T) {
	tx := beginTx(t)
	repo := repository.NewUser(tx)

	first := newUser(t, repo)

	before := countRows(t, tx)

	// 同じ google_sub で 2 人目を作ろうとすると UNIQUE 制約で失敗する
	duplicate := &model.User{
		GoogleSub: first.GoogleSub,
		Email:     "another@example.com",
		Timezone:  "Asia/Tokyo",
	}
	if err := repo.CreateWithInitialData(t.Context(), duplicate); err == nil {
		t.Fatal("重複する google_sub でエラーになりませんでした")
	}

	after := countRows(t, tx)

	for table, count := range after {
		if count != before[table] {
			t.Errorf("%s の件数が変化しました: %d → %d（失敗した分が残っています）",
				table, before[table], count)
		}
	}

	// 1 人目のカテゴリは失敗の巻き添えで消えていないこと
	var firstCategories int64
	err := tx.Model(&model.Category{}).Where("user_id = ?", first.ID).Count(&firstCategories).Error
	if err != nil {
		t.Fatalf("カテゴリ数の取得に失敗しました: %v", err)
	}
	if firstCategories != 9 {
		t.Errorf("1 人目のカテゴリ数: got %d, want 9", firstCategories)
	}
}

// countRows は users / categories / payment_methods の件数をまとめて取る。
func countRows(t *testing.T, tx *gorm.DB) map[string]int64 {
	t.Helper()

	counts := make(map[string]int64, 3)
	for table, dest := range map[string]any{
		"users":           &model.User{},
		"categories":      &model.Category{},
		"payment_methods": &model.PaymentMethod{},
	} {
		var count int64
		if err := tx.Model(dest).Count(&count).Error; err != nil {
			t.Fatalf("%s の件数取得に失敗しました: %v", table, err)
		}
		counts[table] = count
	}
	return counts
}

func TestUser_FindByGoogleSub(t *testing.T) {
	tx := beginTx(t)
	repo := repository.NewUser(tx)

	created := newUser(t, repo)

	t.Run("存在すれば取得できる", func(t *testing.T) {
		found, err := repo.FindByGoogleSub(t.Context(), created.GoogleSub)
		if err != nil {
			t.Fatalf("取得に失敗しました: %v", err)
		}
		if found.ID != created.ID {
			t.Errorf("ID: got %d, want %d", found.ID, created.ID)
		}
	})

	t.Run("存在しなければ ErrNotFound", func(t *testing.T) {
		_, err := repo.FindByGoogleSub(t.Context(), "no-such-sub")
		if !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("got %v, want ErrNotFound", err)
		}
	})
}

func TestUser_FindByID(t *testing.T) {
	tx := beginTx(t)
	repo := repository.NewUser(tx)

	created := newUser(t, repo)

	t.Run("存在すれば取得できる", func(t *testing.T) {
		found, err := repo.FindByID(t.Context(), created.ID)
		if err != nil {
			t.Fatalf("取得に失敗しました: %v", err)
		}
		if found.Email != created.Email {
			t.Errorf("Email: got %q, want %q", found.Email, created.Email)
		}
	})

	t.Run("存在しなければ ErrNotFound", func(t *testing.T) {
		if _, err := repo.FindByID(t.Context(), 999999); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("got %v, want ErrNotFound", err)
		}
	})
}

// TestUser_Exists は認証ミドルウェアが使うユーザの存在確認を検証する。
//
// 署名の正しいトークンでもユーザが消えていれば通してはいけない。
// ここが常に true を返すと、消えたユーザの ID が下流に渡り、
// 取引やカテゴリの作成が外部キー違反の 500 になる。
func TestUser_Exists(t *testing.T) {
	tx := beginTx(t)
	repo := repository.NewUser(tx)

	created := newUser(t, repo)

	t.Run("存在すれば true", func(t *testing.T) {
		exists, err := repo.Exists(t.Context(), created.ID)
		if err != nil {
			t.Fatalf("存在確認に失敗しました: %v", err)
		}
		if !exists {
			t.Error("got false, want true")
		}
	})

	t.Run("存在しなければ false（エラーにはしない）", func(t *testing.T) {
		exists, err := repo.Exists(t.Context(), 999999)
		if err != nil {
			t.Fatalf("存在確認に失敗しました: %v", err)
		}
		if exists {
			t.Error("got true, want false")
		}
	})
}

// TestUser_UpdateProfile は Google 側で変わったメールアドレス・表示名が
// ログインのたびに同期されることを確認する。
func TestUser_UpdateProfile(t *testing.T) {
	tx := beginTx(t)
	repo := repository.NewUser(tx)

	user := newUser(t, repo)

	newName := "改名後"
	user.Email = "changed@example.com"
	user.DisplayName = &newName

	if err := repo.UpdateProfile(t.Context(), user); err != nil {
		t.Fatalf("更新に失敗しました: %v", err)
	}

	found, err := repo.FindByID(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("取得に失敗しました: %v", err)
	}
	if found.Email != "changed@example.com" {
		t.Errorf("Email: got %q, want %q", found.Email, "changed@example.com")
	}
	if found.DisplayName == nil || *found.DisplayName != newName {
		t.Errorf("DisplayName: got %v, want %q", found.DisplayName, newName)
	}
}

// TestUser_SummarizeMailAccounts は API-004 が返す連携数と再認証フラグを確認する。
//
// このクエリは集計関数とプレースホルダを組み合わせており、
// GORM の Select の使い方を誤ると SQL 構文エラーになる。
// 単体テストでは検出できないため、実データベースに対して検証する。
func TestUser_SummarizeMailAccounts(t *testing.T) {
	tests := []struct {
		name            string
		statuses        []string
		wantCount       int
		wantNeedsReauth bool
	}{
		{
			// ログイン直後の状態。連携が 0 件でもエラーにならないこと
			name:      "連携が 0 件",
			statuses:  nil,
			wantCount: 0,
		},
		{
			name:      "active が 1 件",
			statuses:  []string{model.MailAccountStatusActive},
			wantCount: 1,
		},
		{
			name: "reauth_required があれば needs_reauth が true",
			statuses: []string{
				model.MailAccountStatusActive,
				model.MailAccountStatusReauthRequired,
			},
			wantCount:       2,
			wantNeedsReauth: true,
		},
		{
			// 解除済みは「連携している」とは言えないため件数から除く
			name: "disabled は件数に含めない",
			statuses: []string{
				model.MailAccountStatusActive,
				model.MailAccountStatusDisabled,
			},
			wantCount: 1,
		},
		{
			name:      "disabled だけなら 0 件",
			statuses:  []string{model.MailAccountStatusDisabled},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := beginTx(t)
			repo := repository.NewUser(tx)

			user := newUser(t, repo)
			createMailAccounts(t, tx, user.ID, tt.statuses)

			summary, err := repo.SummarizeMailAccounts(t.Context(), user.ID)
			if err != nil {
				t.Fatalf("集計に失敗しました: %v", err)
			}

			if summary.Count != tt.wantCount {
				t.Errorf("Count: got %d, want %d", summary.Count, tt.wantCount)
			}
			if summary.NeedsReauth != tt.wantNeedsReauth {
				t.Errorf("NeedsReauth: got %v, want %v", summary.NeedsReauth, tt.wantNeedsReauth)
			}
		})
	}
}

// TestUser_SummarizeMailAccounts_他ユーザの連携を数えない はマルチテナントの分離を確認する。
func TestUser_SummarizeMailAccounts_他ユーザの連携を数えない(t *testing.T) {
	tx := beginTx(t)
	repo := repository.NewUser(tx)

	me := newUser(t, repo)

	otherName := "他人"
	other := &model.User{
		GoogleSub:   uniqueSub(t) + "-other",
		Email:       uniqueSub(t) + "-other@example.com",
		DisplayName: &otherName,
		Timezone:    "Asia/Tokyo",
	}
	if err := repo.CreateWithInitialData(t.Context(), other); err != nil {
		t.Fatalf("他ユーザの作成に失敗しました: %v", err)
	}

	createMailAccounts(t, tx, other.ID, []string{
		model.MailAccountStatusActive,
		model.MailAccountStatusReauthRequired,
	})

	summary, err := repo.SummarizeMailAccounts(t.Context(), me.ID)
	if err != nil {
		t.Fatalf("集計に失敗しました: %v", err)
	}

	if summary.Count != 0 {
		t.Errorf("Count: got %d, want 0（他ユーザの連携を数えています）", summary.Count)
	}
	if summary.NeedsReauth {
		t.Error("NeedsReauth が true です（他ユーザの状態を見ています）")
	}
}

func createMailAccounts(t *testing.T, tx *gorm.DB, userID int64, statuses []string) {
	t.Helper()

	for i, status := range statuses {
		account := &model.MailAccount{
			UserID:       userID,
			EmailAddress: fmt.Sprintf("mail-%d@example.com", i),
			Provider:     "gmail_api",
			Status:       status,
		}
		if err := tx.WithContext(t.Context()).Create(account).Error; err != nil {
			t.Fatalf("連携メールアカウントの作成に失敗しました: %v", err)
		}
	}
}
