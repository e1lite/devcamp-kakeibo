"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useMemo, useRef } from "react";

import { Page } from "@/components/app/AppShell";
import { LinkButton } from "@/components/Button";
import { EmptyState, LoadError, Skeleton } from "@/components/Feedback";
import { Pagination } from "@/features/transactions/components/Pagination";
import { TransactionFilters } from "@/features/transactions/components/TransactionFilters";
import { TransactionRows } from "@/features/transactions/components/TransactionRows";
import { useTransactionList } from "@/features/transactions/hooks/useTransactionList";
import {
  effectivePeriod,
  hasConditions,
  parseListQuery,
  toListHref,
  withFilter,
  type ListFilter,
} from "@/features/transactions/listQuery";
import { listCategories, listPaymentMethods } from "@/lib/api/endpoints";
import { formatPeriod } from "@/lib/date";
import { formatYen } from "@/lib/format";
import { useResource } from "@/lib/useResource";

// 選択肢の取得は画面を開いたときに 1 回だけ。条件を変えたときは API-010 だけを呼ぶ（SCR-010 #1）
const loadCategories = () => listCategories();
// 過去の取引を絞り込めるよう、利用停止中の決済手段も含める（イベント設計 0 章 #4）
const loadPaymentMethods = () => listPaymentMethods({ includeInactive: true });

/** SCR-010 取引一覧。開いた瞬間に「今月いくら使ったか」が分かることを最優先にする。 */
export function TransactionListView() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const query = useMemo(() => parseListQuery(new URLSearchParams(searchParams.toString())), [searchParams]);

  const { state, retry } = useTransactionList(query);
  const categories = useResource(loadCategories);
  const paymentMethods = useResource(loadPaymentMethods);
  const listTopRef = useRef<HTMLDivElement>(null);

  // 絞り込みの操作は URL を書き換えるだけ（SCR-010 #2〜#5）
  const changeFilter = (changes: Partial<ListFilter>) => {
    router.push(toListHref(withFilter(query, changes)), { scroll: false });
  };

  const scrollToListTop = () => listTopRef.current?.scrollIntoView({ block: "start" });

  // 「店舗: ○○」の名前は、取得した一覧の merchant.name から取る
  const merchantName =
    state.status === "success" && query.merchantId
      ? (state.rows.find((row) => String(row.merchant?.id) === query.merchantId)?.merchant?.name ?? null)
      : null;

  const period = effectivePeriod(query);

  return (
    <Page width="list">
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-2xl font-bold">取引</h1>
        {/* 主ボタン。PC は見出しの右、スマホは画面下部に固定（下の fixed） */}
        <LinkButton href="/transactions/new" variant="primary" className="max-md:hidden">
          ＋ 登録
        </LinkButton>
      </div>

      <TransactionFilters
        query={query}
        onChange={changeFilter}
        categories={categories}
        paymentMethods={paymentMethods}
        merchantName={merchantName}
      />

      <div ref={listTopRef} className="flex scroll-mt-20 flex-col gap-4" aria-live="polite" aria-busy={state.status === "loading"}>
        {state.status === "loading" && (
          <>
            <Skeleton rows={1} className="max-w-xs" />
            <div className="rounded-md border border-line bg-surface p-4">
              <Skeleton rows={6} />
            </div>
          </>
        )}

        {state.status === "error" && <LoadError message="取引を読み込めませんでした。" onRetry={retry} />}

        {state.status === "invalid" && (
          <EmptyState
            title="条件が正しくありません"
            action={
              <LinkButton href="/transactions" variant="secondary">
                条件をクリア
              </LinkButton>
            }
          />
        )}

        {state.status === "success" && (
          <>
            {/* C 集計。一覧より上に、一覧より大きな文字で出す */}
            <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
              <p className="amount text-2xl font-bold">{state.meta.total} 件</p>
              <p className="amount text-2xl font-bold">合計 {formatYen(state.meta.total_amount_minor)}</p>
              <p className="text-sm text-muted">{formatPeriod(period.from, period.to)}</p>
            </div>

            {state.rows.length > 0 ? (
              <>
                <TransactionRows rows={state.rows} />
                <Pagination query={query} total={state.meta.total} onNavigate={scrollToListTop} />
              </>
            ) : query.page >= 2 ? (
              // 削除後に戻った場合など、ページが範囲外
              <EmptyState
                title="このページに取引はありません"
                action={
                  <LinkButton href={toListHref({ ...query, page: 1 })} variant="secondary">
                    1 ページ目へ
                  </LinkButton>
                }
              />
            ) : hasConditions(query) ? (
              // 絞り込んだ結果の 0 件。次に取ってほしい行動は「条件を外す」
              <EmptyState
                title="条件に合う取引はありません"
                action={
                  <LinkButton href="/transactions" variant="secondary">
                    条件をクリア
                  </LinkButton>
                }
              />
            ) : (
              // 初めて使う人がまず見る 0 件。次に取ってほしい行動は「登録する」
              <EmptyState
                title="まだ取引がありません"
                description="現金で払った支出は「登録」から入力できます。"
                action={
                  <LinkButton href="/transactions/new" variant="primary">
                    ＋ 登録
                  </LinkButton>
                }
              />
            )}
          </>
        )}
      </div>

      {/* スマホの主ボタンは、片手の親指で押せる画面下部に固定する */}
      <div className="fixed inset-x-0 bottom-0 z-10 border-t border-line bg-surface p-4 md:hidden">
        <LinkButton href="/transactions/new" variant="primary" className="w-full">
          ＋ 登録
        </LinkButton>
      </div>
    </Page>
  );
}
