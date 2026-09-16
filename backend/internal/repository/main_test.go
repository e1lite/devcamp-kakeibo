package repository_test

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/database"
)

// testDB は全テストで共有する接続。TestMain で 1 度だけ開く。
var testDB *gorm.DB

// TestMain は実データベースへの接続を用意する。
//
// このパッケージのテストは PostgreSQL に接続する統合テストであり、
// マイグレーション済みのデータベースが起動している必要がある（make setup）。
// GORM の API の使い方の誤りは SQL を実行して初めて表面化するため、
// リポジトリ層は実データベースに対して検証する。
//
//	go test ./...           統合テストを含めて実行する
//	go test -short ./...    データベースを使うテストをスキップする
func TestMain(m *testing.M) {
	// testing.Short() はフラグ解析後でないと使えない
	flag.Parse()

	if testing.Short() {
		os.Exit(m.Run())
	}

	db, err := openTestDB()
	if err != nil {
		log.Fatalf("テスト用データベースに接続できません: %v\n"+
			"'make setup' でデータベースを起動してください。"+
			"データベースを使わない場合は 'go test -short ./...' を実行してください。", err)
	}
	testDB = db

	code := m.Run()

	if cerr := database.Close(testDB); cerr != nil {
		log.Printf("接続のクローズに失敗しました: %v", cerr)
	}
	os.Exit(code)
}

func openTestDB() (*gorm.DB, error) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		return nil, fmt.Errorf("TEST_DATABASE_URL も DATABASE_URL も設定されていません")
	}

	db, err := database.Open(dsn, false)
	if err != nil {
		return nil, err
	}

	// 接続できるかをここで確かめる。Open は遅延接続のため、
	// データベースが起動していなくても成功してしまう
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := database.Ping(ctx, db); err != nil {
		return nil, err
	}
	return db, nil
}

// beginTx はテスト 1 件分のトランザクションを開始し、終了時にロールバックする。
//
// テストが書いたデータは一切コミットされないため、開発用データベースを
// そのまま使っても中身が汚れない。リポジトリ内の Transaction は
// GORM がセーブポイントに読み替えるため、入れ子でも動作する。
func beginTx(t *testing.T) *gorm.DB {
	t.Helper()

	if testDB == nil {
		t.Skip("-short が指定されたため、データベースを使うテストをスキップします")
	}

	tx := testDB.Begin()
	if tx.Error != nil {
		t.Fatalf("トランザクションの開始に失敗しました: %v", tx.Error)
	}

	t.Cleanup(func() {
		if err := tx.Rollback().Error; err != nil {
			t.Errorf("ロールバックに失敗しました: %v", err)
		}
	})

	return tx
}
