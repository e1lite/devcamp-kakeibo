import type { ReactNode } from "react";

import { Button } from "@/components/Button";

// 状態の表示（Step 5 のレイアウトの方針「状態は、変わった場所の中で示す」）。

/** 読み込み中のスケルトン。行の形の灰色の帯を並べる。 */
export function Skeleton({ rows = 3, className = "" }: { rows?: number; className?: string }) {
  return (
    <div className={`flex flex-col gap-3 ${className}`} aria-busy="true" aria-label="読み込み中">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="h-5 animate-pulse rounded bg-skeleton" style={{ width: `${90 - (i % 3) * 15}%` }} />
      ))}
    </div>
  );
}

/**
 * 画面を開いたときの取得が 5xx・通信エラーで失敗したとき（イベント設計 1.2）。
 * 取得できなかった領域の中だけに出し、「再試行」で同じ API を呼び直す。
 */
export function LoadError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div role="alert" className="flex flex-wrap items-center gap-3 rounded-md border border-line bg-surface p-4">
      <p className="text-sm">{message}</p>
      <Button variant="secondary" onClick={onRetry}>
        再試行
      </Button>
    </div>
  );
}

/**
 * 操作（登録・更新・削除）のエラー。画面の内容の先頭に出し、消えない（共通レイアウトの B メッセージ）。
 */
export function ErrorBanner({ children }: { children: ReactNode }) {
  return (
    <div role="alert" className="rounded-md border border-danger bg-danger-soft px-4 py-3 text-sm text-danger">
      {children}
    </div>
  );
}

/** 一覧の 0 件など、次にできることを添えて伝える枠。 */
export function EmptyState({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-md border border-dashed border-line px-4 py-10 text-center">
      <p className="font-bold">{title}</p>
      {description && <p className="text-sm text-muted">{description}</p>}
      {action}
    </div>
  );
}

/**
 * 操作のエラーの文言。5xx・通信エラーは原因と次にすることを書く（共通レイアウトの状態
 * 「操作のエラー」の例: 保存できませんでした。通信状況を確認して、もう一度「登録」を押してください。）
 */
export function operationErrorMessage(button: string): string {
  return `保存できませんでした。通信状況を確認して、もう一度「${button}」を押してください。`;
}
