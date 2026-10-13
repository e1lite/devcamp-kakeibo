# frontend

決済メール解析による自動家計簿サービスのフロントエンド（Step 6）。Next.js 16（App Router）+ TypeScript + Tailwind CSS。

画面は Step 4・5 の設計に沿って作っている。

| 設計 | 場所 |
|---|---|
| 画面一覧・画面遷移図 | [docs/step4-screens.md](../docs/step4-screens.md)・[docs/step4-transition.md](../docs/step4-transition.md) |
| 画面レイアウト | [docs/step5-layouts.html](../docs/step5-layouts.html) |
| イベント設計 | [docs/step5-events.md](../docs/step5-events.md) |

## 起動

API サーバ（`localhost:8080`）を先に起動しておく（リポジトリ直下で `make setup` → `make run`）。

```bash
npm ci
npm run dev      # http://localhost:5173
```

ブラウザからは `/api/*` を同じオリジン（`localhost:5173`）として呼び、Next.js が API サーバへ転送する
（`next.config.ts` の rewrites。転送先は環境変数 `API_ORIGIN`、既定 `http://localhost:8080`）。
API サーバは CORS ヘッダを返さないため、この形にしている（API 仕様書 2 章）。

## コマンド

| コマンド | 内容 |
|---|---|
| `npm run dev` | 開発サーバ（ポート 5173） |
| `npm run build` | 本番ビルド |
| `npm run typecheck` | TypeScript の型チェック（`tsc --noEmit`） |
| `npm run lint` | ESLint（Next.js の推奨設定 + React Hooks / React Compiler の規則） |
| `npm run test` | 単体テスト（Vitest） |
| `npm run check` | 型チェック・ESLint・単体テストをまとめて実行する。コミット前に通す |

## 構成

```
src/
├── app/                      画面（URL と 1 対 1）
│   ├── page.tsx              SCR-001 ルート（振り分けのみ）
│   ├── login/                SCR-002 ログイン
│   ├── auth/callback/        SCR-003 ログイン処理中
│   ├── (app)/                ログインが必要な画面。layout.tsx が共通レイアウト（ヘッダー・ログインの確認）
│   │   ├── transactions/     SCR-010 取引一覧・new/ SCR-011 登録・[id]/ SCR-012 詳細・[id]/edit/ SCR-013 編集
│   │   └── settings/categories/  SCR-020 カテゴリ管理
│   ├── not-found.tsx         SCR-099 エラー（存在しない URL）
│   └── error.tsx             SCR-099 エラー（想定外の例外）
├── components/               画面をまたいで使う部品（ボタン・入力欄・ダイアログ・共通レイアウト）
├── features/                 画面ごとの処理（取引一覧の URL ↔ API の変換、フォームの検証など）
└── lib/                      API の呼び出しと型・トークンの保存・日時（JST）の変換
```

- **API の呼び出しは `src/lib/api/` の 1 か所にまとめている。** トークンの付与、`401` のときにログイン画面へ送る処理
  （複数の API が同時に `401` を返しても遷移は 1 回だけ）、エラーの形の変換を各画面に書かない
- **取引一覧の取得は `useTransactionList` の 1 か所だけ。** 絞り込みの操作は URL を書き換えるだけにしている。
  理由は [src/features/transactions/README.md](src/features/transactions/README.md)
- ログインのトークンは `sessionStorage` に置く（Step 4 判断 9）。新しいタブではログインし直しになる
- 単体テストは、画面に依存しない純粋な関数（URL と API の値の変換、入力の検証、送る値の組み立て、日時の変換）を対象にしている
