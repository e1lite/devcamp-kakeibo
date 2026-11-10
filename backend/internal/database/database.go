// Package database は PostgreSQL への接続を扱う。
package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// maxIdleConns は使っていない接続を手元に残す数の上限。
const maxIdleConns = 5

// Open は GORM 経由で PostgreSQL に接続する。
//
// マイグレーションは cmd/migrate が担当するため、ここでは AutoMigrate を呼ばない。
// スキーマ変更の履歴を migrations/ の SQL ファイルとして残し、
// Step 7 以降の本番環境でも同じ手順を踏めるようにするため。
//
// maxOpenConns はこのプロセスが同時に持つ接続の上限。API サーバを複数台で動かすと、
// 台数 × 上限が DB（RDS）の接続数の上限に収まる必要があるため、設定で変えられるようにしている
// （docs/step7-infra.md 6 章 #3）。
func Open(dsn string, verboseLog bool, maxOpenConns int) (*gorm.DB, error) {
	logLevel := logger.Warn
	if verboseLog {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		// 日時は timestamptz で保持し、集計は JST 基準で行う（DB 仕様書 1 章）。
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, fmt.Errorf("データベースへの接続に失敗しました: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("コネクションプールの取得に失敗しました: %w", err)
	}
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(min(maxIdleConns, maxOpenConns))
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

// Ping はデータベースへ疎通できるか確認する。ヘルスチェック（API-001）が使う。
func Ping(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("コネクションプールの取得に失敗しました: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("データベースへ疎通できません: %w", err)
	}
	return nil
}

// Close はコネクションプールを閉じる。
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("コネクションプールの取得に失敗しました: %w", err)
	}
	return sqlDB.Close()
}
