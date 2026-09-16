// Command migrate は migrations/ の SQL ファイルを適用する。
//
// GORM の AutoMigrate ではなく SQL ファイルによるマイグレーションにしているのは、
// スキーマ変更の履歴を追える形で残し、Step 7 以降の本番環境でも
// 同じ手順を踏めるようにするため。
//
//	go run ./cmd/migrate up          最新まで適用する
//	go run ./cmd/migrate down 1      1 つ戻す
//	go run ./cmd/migrate version     現在のバージョンを表示する
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

const migrationsPath = "file://migrations"

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("マイグレーションに失敗しました: %v", err)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("コマンドを指定してください（up / down / version）")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL が設定されていません")
	}

	m, err := migrate.New(migrationsPath, dsn)
	if err != nil {
		return fmt.Errorf("マイグレーションの初期化に失敗しました: %w", err)
	}
	defer func() {
		// Close は source と database の 2 つのエラーを返す
		if serr, derr := m.Close(); serr != nil || derr != nil {
			log.Printf("クローズに失敗しました: source=%v database=%v", serr, derr)
		}
	}()

	switch args[0] {
	case "up":
		return applyUp(m)
	case "down":
		return applyDown(m, args[1:])
	case "version":
		return printVersion(m)
	default:
		return fmt.Errorf("不明なコマンドです: %s", args[0])
	}
}

func applyUp(m *migrate.Migrate) error {
	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Println("適用すべきマイグレーションはありません")
			return nil
		}
		return err
	}
	log.Println("マイグレーションを最新まで適用しました")
	return printVersion(m)
}

func applyDown(m *migrate.Migrate, args []string) error {
	// 引数なしの down は全テーブルを消してしまうため、戻す数を必須にする
	if len(args) == 0 {
		return errors.New("戻すマイグレーションの数を指定してください（例: down 1）")
	}

	steps, err := strconv.Atoi(args[0])
	if err != nil || steps < 1 {
		return fmt.Errorf("戻す数は 1 以上の整数で指定してください: %s", args[0])
	}

	if err := m.Steps(-steps); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Println("戻すマイグレーションはありません")
			return nil
		}
		return err
	}
	log.Printf("マイグレーションを %d つ戻しました", steps)
	return printVersion(m)
}

func printVersion(m *migrate.Migrate) error {
	v, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		log.Println("現在のバージョン: なし（未適用）")
		return nil
	}
	if err != nil {
		return fmt.Errorf("バージョンの取得に失敗しました: %w", err)
	}

	if dirty {
		// 適用の途中で失敗した状態。手動での復旧が必要になる
		return fmt.Errorf("バージョン %d が dirty です。手動で復旧してください", v)
	}

	// v は uint のため、書式指定 %d に改行や制御文字を注入する余地はない。
	// gosec の taint analysis は DATABASE_URL 由来として警告するが、
	// ログインジェクションは成立しない。
	//nolint:gosec // G706: uint の出力にログインジェクションは成立しない
	log.Printf("現在のバージョン: %d", v)
	return nil
}
