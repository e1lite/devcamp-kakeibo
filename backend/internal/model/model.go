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
