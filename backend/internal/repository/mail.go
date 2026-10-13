package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/model"
)

// MailAccount は mail_accounts テーブルへのアクセスを提供する。
type MailAccount struct {
	db *gorm.DB
}

// NewMailAccount は MailAccount リポジトリを生成する。
func NewMailAccount(db *gorm.DB) *MailAccount {
	return &MailAccount{db: db}
}

// ListByUser はユーザの連携を ID の昇順で返す（API-006）。解除済み（disabled）も含める。
func (r *MailAccount) ListByUser(ctx context.Context, userID int64) ([]model.MailAccount, error) {
	var accounts []model.MailAccount
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("id").Find(&accounts).Error
	if err != nil {
		return nil, fmt.Errorf("連携の一覧の取得に失敗しました: %w", err)
	}
	return accounts, nil
}

// FindByID は連携を取得する。所有者の確認は呼び出し側が行う。
func (r *MailAccount) FindByID(ctx context.Context, id int64) (*model.MailAccount, error) {
	var account model.MailAccount
	if err := r.db.WithContext(ctx).First(&account, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("連携の取得に失敗しました: %w", err)
	}
	return &account, nil
}

// FindByUserAndEmail は同じユーザ・同じアドレスの連携を取得する（再連携のとき）。
func (r *MailAccount) FindByUserAndEmail(ctx context.Context, userID int64, email string) (*model.MailAccount, error) {
	var account model.MailAccount
	err := r.db.WithContext(ctx).Where("user_id = ? AND email_address = ?", userID, email).First(&account).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("連携の取得に失敗しました: %w", err)
	}
	return &account, nil
}

// CountActive は使える状態（active）の連携の数を返す。
func (r *MailAccount) CountActive(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.MailAccount{}).
		Where("user_id = ? AND status = ?", userID, model.MailAccountStatusActive).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("連携の数の取得に失敗しました: %w", err)
	}
	return count, nil
}

// Create は連携を作る。
func (r *MailAccount) Create(ctx context.Context, account *model.MailAccount) error {
	if err := r.db.WithContext(ctx).Create(account).Error; err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateName
		}
		return fmt.Errorf("連携の作成に失敗しました: %w", err)
	}
	return nil
}

// Update は指定した列だけを更新する。
func (r *MailAccount) Update(ctx context.Context, id int64, fields map[string]any) error {
	err := r.db.WithContext(ctx).Model(&model.MailAccount{}).Where("id = ?", id).Updates(fields).Error
	if err != nil {
		return fmt.Errorf("連携の更新に失敗しました: %w", err)
	}
	return nil
}

// SyncJob は sync_jobs テーブルへのアクセスを提供する。
type SyncJob struct {
	db *gorm.DB
}

// NewSyncJob は SyncJob リポジトリを生成する。
func NewSyncJob(db *gorm.DB) *SyncJob {
	return &SyncJob{db: db}
}

// Create はジョブを作る。
func (r *SyncJob) Create(ctx context.Context, job *model.SyncJob) error {
	if err := r.db.WithContext(ctx).Create(job).Error; err != nil {
		return fmt.Errorf("同期ジョブの作成に失敗しました: %w", err)
	}
	return nil
}

// Update は指定した列だけを更新する（ジョブの終了時）。
func (r *SyncJob) Update(ctx context.Context, id int64, fields map[string]any) error {
	err := r.db.WithContext(ctx).Model(&model.SyncJob{}).Where("id = ?", id).Updates(fields).Error
	if err != nil {
		return fmt.Errorf("同期ジョブの更新に失敗しました: %w", err)
	}
	return nil
}

// HasRunning は連携に実行中のジョブがあるかを返す（API-008 の 409 SYNC_ALREADY_RUNNING）。
func (r *SyncJob) HasRunning(ctx context.Context, mailAccountID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.SyncJob{}).
		Where("mail_account_id = ? AND status = ?", mailAccountID, model.SyncStatusRunning).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("実行中の同期ジョブの確認に失敗しました: %w", err)
	}
	return count > 0, nil
}

// FailRunning は実行中のまま残ったジョブを失敗にする。
//
// ジョブは API サーバの中で動かすため（docs/mail-import-mvp.md 判断 4）、サーバを止めると
// 実行中のまま残り、同じ連携の同期が 409 SYNC_ALREADY_RUNNING で始められなくなる。
// 起動時に呼んで、中断されたことを記録する。
func (r *SyncJob) FailRunning(ctx context.Context, now time.Time, message string) (int64, error) {
	result := r.db.WithContext(ctx).Model(&model.SyncJob{}).
		Where("status = ?", model.SyncStatusRunning).
		Updates(map[string]any{"status": model.SyncStatusFailed, "finished_at": now, "error_message": message})
	if result.Error != nil {
		return 0, fmt.Errorf("中断された同期ジョブの記録に失敗しました: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// SyncJobFilter は API-009 の絞り込み条件。
type SyncJobFilter struct {
	MailAccountID *int64
	Status        *string
	Limit         int
	Offset        int
}

// List はユーザのジョブを新しい順に返す（API-009）。総件数も返す。
func (r *SyncJob) List(ctx context.Context, userID int64, f SyncJobFilter) ([]model.SyncJob, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.SyncJob{}).
		Joins("JOIN mail_accounts ma ON ma.id = sync_jobs.mail_account_id").
		Where("ma.user_id = ?", userID)
	if f.MailAccountID != nil {
		query = query.Where("sync_jobs.mail_account_id = ?", *f.MailAccountID)
	}
	if f.Status != nil {
		query = query.Where("sync_jobs.status = ?", *f.Status)
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("同期履歴の件数の取得に失敗しました: %w", err)
	}

	var jobs []model.SyncJob
	err := query.Session(&gorm.Session{}).
		Select("sync_jobs.*").
		Order("sync_jobs.started_at DESC, sync_jobs.id DESC").
		Limit(f.Limit).Offset(f.Offset).
		Find(&jobs).Error
	if err != nil {
		return nil, 0, fmt.Errorf("同期履歴の取得に失敗しました: %w", err)
	}
	return jobs, total, nil
}

// EmailMessage は email_messages テーブルへのアクセスを提供する。
type EmailMessage struct {
	db *gorm.DB
}

// NewEmailMessage は EmailMessage リポジトリを生成する。
func NewEmailMessage(db *gorm.DB) *EmailMessage {
	return &EmailMessage{db: db}
}

// ExistingProviderIDs は ids のうち、すでに保存したメールの ID を返す。
// 保存済みのメールは原文を取り直さない（Gmail API の呼び出しを減らす）。
func (r *EmailMessage) ExistingProviderIDs(ctx context.Context, mailAccountID int64, ids []string) (map[string]bool, error) {
	existing := make(map[string]bool, len(ids))
	if len(ids) == 0 {
		return existing, nil
	}
	var found []string
	err := r.db.WithContext(ctx).Model(&model.EmailMessage{}).
		Where("mail_account_id = ? AND provider_message_id IN ?", mailAccountID, ids).
		Pluck("provider_message_id", &found).Error
	if err != nil {
		return nil, fmt.Errorf("保存済みのメールの確認に失敗しました: %w", err)
	}
	for _, id := range found {
		existing[id] = true
	}
	return existing, nil
}

// Insert はメールを保存する。同じメールがすでにあれば何もせず false を返す。
//
// UNIQUE(mail_account_id, provider_message_id) で重複を防ぐ（DB 仕様書 3.4 の取込の冪等性）。
// 同期の期間を重ねて取得しても、同じメールが 2 行にならない。
func (r *EmailMessage) Insert(ctx context.Context, msg *model.EmailMessage) (bool, error) {
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "mail_account_id"}, {Name: "provider_message_id"}},
			DoNothing: true,
		}).
		Create(msg)
	if result.Error != nil {
		return false, fmt.Errorf("メールの保存に失敗しました: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}
