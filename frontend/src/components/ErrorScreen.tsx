import Link from "next/link";

import { buttonClass } from "@/components/Button";

// SCR-099 エラー。画面の中央に 1 つだけ置く（Step 5 の画面レイアウト SCR-099）。
// 共通ヘッダーは出さない（ログインしていなくても表示する画面のため）。

type ErrorScreenProps = {
  status: 404 | 500;
  title: string;
  description: string;
};

export const NOT_FOUND_PAGE = {
  status: 404,
  title: "ページが見つかりません",
  description: "URL が間違っているか、ページが削除された可能性があります。",
} as const;

// 存在しない・削除済みの取引に加え、他のユーザの取引も同じ表示にする（Step 4 のフィードバック）
export const NOT_FOUND_TRANSACTION = {
  status: 404,
  title: "取引が見つかりません",
  description: "削除されたか、URL が間違っている可能性があります。",
} as const;

export const UNEXPECTED_ERROR = {
  status: 500,
  title: "問題が発生しました",
  description: "時間をおいて、もう一度お試しください。",
} as const;

export function ErrorBox({ status, title, description }: ErrorScreenProps) {
  return (
    <div className="mx-auto flex w-full max-w-sm flex-col items-center gap-3 rounded-lg border border-line bg-surface p-8 text-center">
      <p className="amount text-sm text-muted">{status}</p>
      <h1 className="text-xl font-bold">{title}</h1>
      <p className="text-sm text-muted">{description}</p>
      {/* ログインしていなければ、遷移先でログイン画面へ送られる（SCR-099 #2） */}
      <Link href="/transactions" className={buttonClass("primary", "mt-2")}>
        取引一覧へ
      </Link>
    </div>
  );
}

export function ErrorScreen(props: ErrorScreenProps) {
  return (
    <main className="flex min-h-dvh items-center justify-center px-4">
      <ErrorBox {...props} />
    </main>
  );
}
