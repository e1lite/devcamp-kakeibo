package repository_test

import (
	"testing"
	"time"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/model"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/repository"
)

func newMailAccount(t *testing.T, repo *repository.MailAccount, userID int64) *model.MailAccount {
	t.Helper()

	account := &model.MailAccount{
		UserID:              userID,
		EmailAddress:        uniqueSub(t) + "@gmail.example.com",
		Provider:            model.MailAccountProviderGmailAPI,
		CredentialEncrypted: "encrypted",
		Status:              model.MailAccountStatusActive,
	}
	if err := repo.Create(t.Context(), account); err != nil {
		t.Fatalf("連携の作成に失敗しました: %v", err)
	}
	return account
}

func newEmail(userID, accountID int64, providerID string) *model.EmailMessage {
	subject := "カードご利用のお知らせ"
	return &model.EmailMessage{
		UserID:            userID,
		MailAccountID:     accountID,
		ProviderMessageID: providerID,
		FromAddress:       "info@card.example.jp",
		Subject:           &subject,
		ReceivedAt:        time.Date(2026, 10, 6, 12, 35, 0, 0, jst),
		Classification:    "unknown",
		ParseStatus:       model.ParseStatusPending,
	}
}

// TestEmailMessage_Insert_同じメールは1行にしかならない は取込の冪等性の中心を確認する。
//
// 同期は期間を重ねて取得する（前回の同期の 1 日前から）ため、同じメールを何度も受け取る。
// UNIQUE(mail_account_id, provider_message_id) で、2 回目以降は保存しない（DB 仕様書 3.4）。
func TestEmailMessage_Insert_同じメールは1行にしかならない(t *testing.T) {
	tx := beginTx(t)
	user := newUser(t, repository.NewUser(tx))
	account := newMailAccount(t, repository.NewMailAccount(tx), user.ID)
	repo := repository.NewEmailMessage(tx)

	first, err := repo.Insert(t.Context(), newEmail(user.ID, account.ID, "gmail-1"))
	if err != nil || !first {
		t.Fatalf("1 回目: saved=%v err=%v", first, err)
	}

	second, err := repo.Insert(t.Context(), newEmail(user.ID, account.ID, "gmail-1"))
	if err != nil {
		t.Fatalf("2 回目でエラーになりました（重複は無視するはず）: %v", err)
	}
	if second {
		t.Error("同じメールが 2 回保存されました")
	}

	var count int64
	if cerr := tx.Model(&model.EmailMessage{}).Where("mail_account_id = ?", account.ID).Count(&count).Error; cerr != nil {
		t.Fatalf("件数の取得に失敗しました: %v", cerr)
	}
	if count != 1 {
		t.Errorf("件数: got %d, want 1", count)
	}

	existing, err := repo.ExistingProviderIDs(t.Context(), account.ID, []string{"gmail-1", "gmail-2"})
	if err != nil {
		t.Fatalf("保存済みの確認に失敗しました: %v", err)
	}
	if !existing["gmail-1"] || existing["gmail-2"] {
		t.Errorf("保存済みの判定: got %v, want gmail-1 だけ", existing)
	}
}

// TestSyncJob_実行中と中断の記録 は、同じ連携の同期を重ねないための判定と、
// サーバの停止で中断されたジョブの後始末を確認する。
func TestSyncJob_実行中と中断の記録(t *testing.T) {
	tx := beginTx(t)
	user := newUser(t, repository.NewUser(tx))
	account := newMailAccount(t, repository.NewMailAccount(tx), user.ID)
	repo := repository.NewSyncJob(tx)

	running, err := repo.HasRunning(t.Context(), account.ID)
	if err != nil || running {
		t.Fatalf("ジョブが無いのに実行中と判定しました: running=%v err=%v", running, err)
	}

	job := &model.SyncJob{
		MailAccountID: account.ID,
		Trigger:       model.SyncTriggerManual,
		StartedAt:     time.Now(),
		Status:        model.SyncStatusRunning,
	}
	if cerr := repo.Create(t.Context(), job); cerr != nil {
		t.Fatalf("ジョブの作成に失敗しました: %v", cerr)
	}

	if running, _ := repo.HasRunning(t.Context(), account.ID); !running {
		t.Error("実行中のジョブを見落としています（同期が重なって始まってしまう）")
	}

	// 起動時の後始末。実行中のまま残ったジョブを失敗にする
	if _, ferr := repo.FailRunning(t.Context(), time.Now(), "中断"); ferr != nil {
		t.Fatalf("中断の記録に失敗しました: %v", ferr)
	}
	if running, _ := repo.HasRunning(t.Context(), account.ID); running {
		t.Error("中断したジョブが実行中のまま残っています")
	}

	jobs, total, err := repo.List(t.Context(), user.ID, repository.SyncJobFilter{Limit: 20})
	if err != nil {
		t.Fatalf("履歴の取得に失敗しました: %v", err)
	}
	if total != 1 || len(jobs) != 1 || jobs[0].Status != model.SyncStatusFailed {
		t.Errorf("履歴: total=%d jobs=%+v", total, jobs)
	}

	// 他ユーザの履歴は見えない
	other := &model.User{
		GoogleSub: uniqueSub(t) + "-other",
		Email:     uniqueSub(t) + "-other@example.com",
		Timezone:  "Asia/Tokyo",
	}
	if err := repository.NewUser(tx).CreateWithInitialData(t.Context(), other); err != nil {
		t.Fatalf("他ユーザの作成に失敗しました: %v", err)
	}
	if _, total, _ := repo.List(t.Context(), other.ID, repository.SyncJobFilter{Limit: 20}); total != 0 {
		t.Errorf("他ユーザの履歴が見えています: total=%d", total)
	}
}
