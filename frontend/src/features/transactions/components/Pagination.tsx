import Link from "next/link";

import { PAGE_SIZE, toListHref, type ListQuery } from "@/features/transactions/listQuery";

// 取引一覧の E ページ送り（イベント設計 SCR-010 #7）。20 件ずつ。
// PC はページ番号、スマホは「前へ / 次へ」と現在のページ。
// 何件目を見ているかも示し、最初・最後のページでは「前へ」「次へ」を押せなくする。

type Props = {
  query: ListQuery;
  total: number;
  // ページを移ったら一覧の先頭までスクロールする
  onNavigate: () => void;
};

const itemClass = "inline-flex min-h-11 min-w-11 items-center justify-center rounded-md px-3 text-sm";

/** 表示するページ番号。多いときは先頭・末尾・今のページの前後だけを出し、間を … にする。 */
export function pageNumbers(current: number, last: number): (number | "gap")[] {
  if (last <= 7) return Array.from({ length: last }, (_, i) => i + 1);
  const pages = new Set([1, last, current - 1, current, current + 1]);
  const sorted = [...pages].filter((p) => p >= 1 && p <= last).sort((a, b) => a - b);
  const result: (number | "gap")[] = [];
  for (const page of sorted) {
    const previous = result[result.length - 1];
    if (typeof previous === "number" && page - previous > 1) result.push("gap");
    result.push(page);
  }
  return result;
}

export function Pagination({ query, total, onNavigate }: Props) {
  const last = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const current = query.page;
  if (total <= PAGE_SIZE && current === 1) return null;

  const first = (current - 1) * PAGE_SIZE + 1;
  const end = Math.min(current * PAGE_SIZE, total);
  const href = (page: number) => toListHref({ ...query, page });

  const step = (page: number, label: string, disabled: boolean) =>
    disabled ? (
      <span className={`${itemClass} text-muted opacity-50`} aria-disabled="true">
        {label}
      </span>
    ) : (
      <Link href={href(page)} scroll={false} onClick={onNavigate} className={`${itemClass} border border-line bg-surface`}>
        {label}
      </Link>
    );

  return (
    <nav aria-label="ページ送り" className="flex flex-col items-center gap-2 md:flex-row md:justify-between">
      <p className="amount hidden text-sm text-muted md:block">
        {first}〜{end} 件目 / {total} 件
      </p>
      <div className="flex items-center gap-1">
        {step(current - 1, "‹ 前へ", current <= 1)}

        <span className="amount px-3 text-sm md:hidden" aria-current="page">
          {current} / {last}
        </span>
        <ol className="hidden items-center gap-1 md:flex">
          {pageNumbers(current, last).map((page, i) =>
            page === "gap" ? (
              <li key={`gap-${i}`} className="px-1 text-muted" aria-hidden="true">
                …
              </li>
            ) : (
              <li key={page}>
                {page === current ? (
                  <span className={`${itemClass} amount bg-ink font-bold text-surface`} aria-current="page">
                    {page}
                  </span>
                ) : (
                  <Link href={href(page)} scroll={false} onClick={onNavigate} className={`${itemClass} amount hover:bg-bg`}>
                    {page}
                  </Link>
                )}
              </li>
            ),
          )}
        </ol>

        {step(current + 1, "次へ ›", current >= last)}
      </div>
    </nav>
  );
}
