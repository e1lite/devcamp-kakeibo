// Package model はデータベースのテーブルに対応する構造体を定義する。
package model

import "time"

// User は users テーブル（DB 仕様書 3.1）。
// 認証は Google OAuth に委譲するためパスワードは保持しない。
type User struct {
	ID          int64   `gorm:"primaryKey"`
	GoogleSub   string  `gorm:"uniqueIndex;size:255;not null"`
	Email       string  `gorm:"uniqueIndex;size:255;not null"`
	DisplayName *string `gorm:"size:100"`
	Timezone    string  `gorm:"size:50;not null;default:Asia/Tokyo"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Category は categories テーブル（DB 仕様書 3.11）。
type Category struct {
	ID        int64  `gorm:"primaryKey"`
	UserID    int64  `gorm:"not null"`
	Name      string `gorm:"size:50;not null"`
	ParentID  *int64
	SortOrder int  `gorm:"not null;default:0"`
	IsSystem  bool `gorm:"not null;default:false"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PaymentMethod は payment_methods テーブル（DB 仕様書 3.10）。
type PaymentMethod struct {
	ID        int64  `gorm:"primaryKey"`
	UserID    int64  `gorm:"not null"`
	Name      string `gorm:"size:100;not null"`
	Kind      string `gorm:"size:20;not null"`
	Issuer    *string
	CardLast4 *string `gorm:"size:4"`
	IsActive  bool    `gorm:"not null;default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// MailAccount は mail_accounts テーブル（DB 仕様書 3.2）。
//
// Step 3 ではログイン時のアカウント情報保持までを扱い、
// メール取得の実行は Step 8 以降に回す。
type MailAccount struct {
	ID                  int64  `gorm:"primaryKey"`
	UserID              int64  `gorm:"not null"`
	EmailAddress        string `gorm:"size:255;not null"`
	Provider            string `gorm:"size:20;not null"`
	CredentialEncrypted string `gorm:"not null;default:''"`
	CredentialExpiresAt *time.Time
	SyncCursor          *string `gorm:"size:255"`
	BackfilledUntil     *time.Time
	LastSyncedAt        *time.Time
	Status              string `gorm:"size:20;not null;default:active"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// MailAccount.Status の値。
const (
	MailAccountStatusActive         = "active"
	MailAccountStatusReauthRequired = "reauth_required"
	MailAccountStatusDisabled       = "disabled"
)

// MailAccountProviderGmailAPI は MailAccount.Provider の値。最小版は Gmail API のみ（docs/mail-import-mvp.md 判断 1）。
const MailAccountProviderGmailAPI = "gmail_api"

// SyncJob は sync_jobs テーブル（DB 仕様書 3.3）。メール取得ジョブの実行履歴。
type SyncJob struct {
	ID            int64  `gorm:"primaryKey"`
	MailAccountID int64  `gorm:"not null"`
	Trigger       string `gorm:"size:20;not null"`
	StartedAt     time.Time
	FinishedAt    *time.Time
	Status        string `gorm:"size:20;not null"`
	FetchedCount  int    `gorm:"not null;default:0"`
	ParsedCount   int    `gorm:"not null;default:0"`
	FailedCount   int    `gorm:"not null;default:0"`
	CursorBefore  *string
	CursorAfter   *string
	ErrorMessage  *string
	CreatedAt     time.Time
}

// SyncJob.Trigger の値。
const (
	SyncTriggerScheduled = "scheduled"
	SyncTriggerManual    = "manual"
	SyncTriggerBackfill  = "backfill"
)

// SyncJob.Status の値。
const (
	SyncStatusRunning = "running"
	SyncStatusSuccess = "success"
	SyncStatusPartial = "partial"
	SyncStatusFailed  = "failed"
)

// EmailMessage は email_messages テーブル（DB 仕様書 3.4）。取得したメールの生データ。
type EmailMessage struct {
	ID                int64  `gorm:"primaryKey"`
	UserID            int64  `gorm:"not null"`
	MailAccountID     int64  `gorm:"not null"`
	ProviderMessageID string `gorm:"size:255;not null"`
	ThreadID          *string
	FromAddress       string `gorm:"size:255;not null"`
	FromName          *string
	Subject           *string
	ReceivedAt        time.Time
	BodyText          *string
	BodyHTML          *string `gorm:"column:body_html"`
	Classification    string  `gorm:"size:20;not null;default:unknown"`
	ClassifiedAt      *time.Time
	MatchedTemplateID *int64
	ParseStatus       string `gorm:"size:20;not null;default:pending"`
	ParseError        *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// EmailMessage.ParseStatus の値。
const (
	ParseStatusPending = "pending"
	ParseStatusSuccess = "success"
	ParseStatusFailed  = "failed"
	ParseStatusSkipped = "skipped"
)

// EmailMessage.Classification の値。
const (
	ClassificationPayment    = "payment"
	ClassificationNotPayment = "not_payment"
	ClassificationUnknown    = "unknown"
)

// ParserTemplate は parser_templates テーブル（DB 仕様書 3.5）。
// 送信元・件名から決済メールかを判定し、本文から項目を抽出するルール。
type ParserTemplate struct {
	ID     int64 `gorm:"primaryKey"`
	UserID *int64
	Name   string `gorm:"size:100;not null"`
	// FromPattern / SubjectPattern は正規表現
	FromPattern     string `gorm:"size:255;not null"`
	SubjectPattern  *string
	EventType       string `gorm:"size:20;not null"`
	PaymentMethodID *int64
	BodyFormat      string `gorm:"size:10;not null;default:text"`
	// ExtractionRules は jsonb。中身は extract.Rules
	ExtractionRules []byte `gorm:"type:jsonb;not null"`
	IsMultiEvent    bool   `gorm:"not null;default:false"`
	Priority        int    `gorm:"not null;default:100"`
	IsActive        bool   `gorm:"not null;default:true"`
	CreatedBy       string `gorm:"size:20;not null;default:user"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// PaymentEvent は payment_events テーブル（DB 仕様書 3.6）。メールから抽出した決済イベント。
type PaymentEvent struct {
	ID               int64 `gorm:"primaryKey"`
	UserID           int64 `gorm:"not null"`
	EmailMessageID   int64 `gorm:"not null"`
	ParserTemplateID *int64
	TransactionID    *int64
	LinkType         *string
	LinkedAt         *time.Time
	EventType        string    `gorm:"size:20;not null"`
	OccurredAt       time.Time `gorm:"not null"`
	AmountMinor      int64     `gorm:"not null"`
	Currency         string    `gorm:"size:3;not null;default:JPY"`
	AmountJPYMinor   int64     `gorm:"column:amount_jpy_minor;not null"`
	RawMerchantText  *string
	MerchantID       *int64
	PaymentMethodID  *int64
	CardLast4        *string `gorm:"column:card_last4"`
	NormalizedAt     *time.Time
	NotifiedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Merchant は merchants テーブル（DB 仕様書 3.8）。
// 1 行 = 1 支店とし、BrandName でチェーンを束ねる。
type Merchant struct {
	ID            int64  `gorm:"primaryKey"`
	UserID        int64  `gorm:"not null"`
	Name          string `gorm:"size:255;not null"`
	BrandName     *string
	IsOnline      bool `gorm:"not null;default:false"`
	Address       *string
	Latitude      *float64
	Longitude     *float64
	PlaceID       *string
	PlaceTypes    *string
	GeocodeStatus string `gorm:"size:20;not null;default:pending"`
	GeocodedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Transaction は transactions テーブル（DB 仕様書 3.7）。
// 1 回の支出 = 1 行。このテーブルのみ DeletedAt による論理削除とする。
type Transaction struct {
	ID          int64     `gorm:"primaryKey"`
	UserID      int64     `gorm:"not null"`
	OccurredAt  time.Time `gorm:"not null"`
	AmountMinor int64     `gorm:"not null"`
	Currency    string    `gorm:"size:3;not null;default:JPY"`
	// AmountJPYMinor は円換算額。集計はこの列を使う
	AmountJPYMinor      int64 `gorm:"column:amount_jpy_minor;not null"`
	MerchantID          *int64
	CategoryID          *int64
	PaymentMethodID     *int64
	CategorySource      string `gorm:"size:20;not null;default:default"`
	Source              string `gorm:"size:20;not null"`
	Status              string `gorm:"size:20;not null;default:confirmed"`
	IsUserEdited        bool   `gorm:"not null;default:false"`
	IsPossibleDuplicate bool   `gorm:"not null;default:false"`
	// MergedIntoID は統合先の取引 ID。NULL 以外はマージ済みを意味する
	MergedIntoID *int64
	Note         *string
	// DeletedAt は論理削除の実施日時。参照時は常に NULL で絞る
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Transaction.Source の値。
const (
	TransactionSourceEmail  = "email"
	TransactionSourceManual = "manual"
)

// Transaction.Status の値。
const (
	TransactionStatusPending   = "pending"
	TransactionStatusConfirmed = "confirmed"
)

// Transaction.CategorySource の値。ユーザ修正をルールで上書きしないために持つ。
const (
	CategorySourceRule      = "rule"
	CategorySourcePlaceType = "place_type"
	CategorySourceUser      = "user"
	CategorySourceDefault   = "default"
)

// CurrencyJPY は既定の通貨コード。
const CurrencyJPY = "JPY"
