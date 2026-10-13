"use client";

import { useId, useState, type FormEvent } from "react";

import { Button, LinkButton } from "@/components/Button";
import { inputClass } from "@/components/Field";
import { CategoryOptions, PaymentMethodOptions } from "@/components/Options";
import {
  appliedCount,
  effectivePeriod,
  hasConditions,
  periodError,
  type ListFilter,
  type ListQuery,
} from "@/features/transactions/listQuery";
import type { Category, PaymentMethod } from "@/lib/api/types";
import type { ResourceHandle } from "@/lib/useResource";

// 取引一覧の B 絞り込み。操作はすべて URL を書き換えるだけで、API は呼ばない
// （onChange に渡した関数が router.push する）。

type Props = {
  query: ListQuery;
  onChange: (changes: Partial<ListFilter>) => void;
  categories: ResourceHandle<Category[]>;
  paymentMethods: ResourceHandle<PaymentMethod[]>;
  // 店舗で絞り込み中に「店舗: ○○」と出す名前。取得した一覧の merchant.name から取る
  merchantName: string | null;
};

const selectClass = `${inputClass} pr-8`;

export function TransactionFilters({ query, onChange, categories, paymentMethods, merchantName }: Props) {
  const ids = useId();
  // スマホでは「絞り込み」ボタンで開閉する。PC では常に開いておく
  const [openOnMobile, setOpenOnMobile] = useState(false);
  const count = appliedCount(query);

  return (
    <section aria-label="絞り込み" className="flex flex-col gap-3">
      <button
        type="button"
        className="inline-flex min-h-11 items-center self-start text-sm font-bold md:hidden"
        aria-expanded={openOnMobile}
        aria-controls={`${ids}-panel`}
        onClick={() => setOpenOnMobile((v) => !v)}
      >
        {count > 0 ? `絞り込み（${count}）` : "絞り込み"} {openOnMobile ? "▴" : "▾"}
      </button>

      <div
        id={`${ids}-panel`}
        className={`${openOnMobile ? "flex" : "hidden"} flex-col gap-3 rounded-md border border-line bg-surface p-4 md:flex md:flex-row md:flex-wrap md:items-end`}
      >
        <PeriodInput query={query} onChange={onChange} />

        <div className="flex flex-col gap-1 md:w-44">
          <label htmlFor={`${ids}-category`} className="text-sm font-bold">
            カテゴリ
          </label>
          <div className="flex items-center gap-2">
            <select
              id={`${ids}-category`}
              className={selectClass}
              value={query.categoryId ?? ""}
              disabled={categories.state.status !== "success"}
              onChange={(e) => onChange({ categoryId: e.target.value })}
            >
              <option value="">すべて</option>
              {/* 未分類は category_id = NULL。API には none を渡す */}
              <option value="none">未分類</option>
              {categories.state.status === "success" && <CategoryOptions categories={categories.state.data} />}
            </select>
            {categories.state.status === "error" && <OptionRetry onRetry={categories.retry} />}
          </div>
        </div>

        <div className="flex flex-col gap-1 md:w-44">
          <label htmlFor={`${ids}-payment`} className="text-sm font-bold">
            決済手段
          </label>
          <div className="flex items-center gap-2">
            <select
              id={`${ids}-payment`}
              className={selectClass}
              value={query.paymentMethodId ?? ""}
              disabled={paymentMethods.state.status !== "success"}
              onChange={(e) => onChange({ paymentMethodId: e.target.value })}
            >
              <option value="">すべて</option>
              {paymentMethods.state.status === "success" && <PaymentMethodOptions methods={paymentMethods.state.data} />}
            </select>
            {paymentMethods.state.status === "error" && <OptionRetry onRetry={paymentMethods.retry} />}
          </div>
        </div>

        {/* URL の q が変わったら（戻る・条件のクリア）入力欄も合わせる */}
        <KeywordInput key={query.q ?? ""} initial={query.q ?? ""} onSearch={(q) => onChange({ q })} />

        <div className="flex flex-wrap items-center gap-3 md:basis-full">
          {query.merchantId && (
            <button
              type="button"
              onClick={() => onChange({ merchantId: undefined })}
              aria-label={`店舗の絞り込みを外す（${merchantName ?? "選択中の店舗"}）`}
              className="inline-flex min-h-11 items-center gap-2 rounded-full border border-line bg-bg px-4 text-sm"
            >
              店舗: {merchantName ?? "選択中の店舗"} <span aria-hidden="true">×</span>
            </button>
          )}
          {hasConditions(query) && (
            <LinkButton href="/transactions" variant="text">
              条件をクリア
            </LinkButton>
          )}
          <Button variant="secondary" className="ml-auto md:hidden" onClick={() => setOpenOnMobile(false)}>
            閉じる
          </Button>
        </div>
      </div>
    </section>
  );
}

/**
 * 期間（イベント設計 SCR-010 #2）。欄から離れたときに URL を書き換える。
 * 開始日が終了日より後なら URL を変えず、欄の下にエラーを出す（一覧は前の条件のまま）。
 */
function PeriodInput({ query, onChange }: { query: ListQuery; onChange: (changes: Partial<ListFilter>) => void }) {
  const ids = useId();
  const effective = effectivePeriod(query);
  const effectiveFrom = effective.from ?? "";
  const effectiveTo = effective.to ?? "";

  const [from, setFrom] = useState(effectiveFrom);
  const [to, setTo] = useState(effectiveTo);
  const [error, setError] = useState<string | null>(null);

  // URL の期間が変わったら（戻る・条件のクリア・店舗の絞り込み）入力欄も合わせる
  const urlKey = `${effectiveFrom}|${effectiveTo}`;
  const [lastUrlKey, setLastUrlKey] = useState(urlKey);
  if (urlKey !== lastUrlKey) {
    setLastUrlKey(urlKey);
    setFrom(effectiveFrom);
    setTo(effectiveTo);
    setError(null);
  }

  // 離れた欄の値は、描画を待たずに欄そのものから読む（入力の直後に離れても新しい値で判定する）
  const commit = (nextFrom: string, nextTo: string) => {
    const message = periodError(nextFrom, nextTo);
    setError(message);
    if (message) return;
    if (nextFrom === effectiveFrom && nextTo === effectiveTo) return;
    onChange({ from: nextFrom, to: nextTo });
  };

  return (
    <fieldset className="flex flex-col gap-1">
      <legend className="mb-1 text-sm font-bold">期間</legend>
      <div className="flex items-center gap-2">
        <input
          type="date"
          aria-label="開始日"
          className={`${inputClass} md:w-40`}
          value={from}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? `${ids}-error` : undefined}
          onChange={(e) => setFrom(e.target.value)}
          onBlur={(e) => commit(e.currentTarget.value, to)}
        />
        <span aria-hidden="true">〜</span>
        <input
          type="date"
          aria-label="終了日"
          className={`${inputClass} md:w-40`}
          value={to}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? `${ids}-error` : undefined}
          onChange={(e) => setTo(e.target.value)}
          onBlur={(e) => commit(from, e.currentTarget.value)}
        />
      </div>
      {error && (
        <p id={`${ids}-error`} className="text-sm text-danger">
          ⚠ {error}
        </p>
      )}
    </fieldset>
  );
}

/**
 * キーワード検索（イベント設計 SCR-010 #4）。「検索」か Enter で確定する。
 * 1 文字入力するごとには呼ばない（API の呼び出しとブラウザの履歴が入力のたびに増えるため）。
 */
function KeywordInput({ initial, onSearch }: { initial: string; onSearch: (q: string) => void }) {
  const id = useId();
  const [value, setValue] = useState(initial);

  const onSubmit = (event: FormEvent) => {
    event.preventDefault();
    onSearch(value.trim());
  };

  return (
    <form role="search" onSubmit={onSubmit} className="flex flex-col gap-1 md:flex-1">
      <label htmlFor={id} className="text-sm font-bold">
        店舗名・メモで検索
      </label>
      <div className="flex gap-2">
        <input
          id={id}
          type="search"
          className={inputClass}
          value={value}
          onChange={(e) => setValue(e.target.value)}
        />
        <Button type="submit" variant="secondary">
          検索
        </Button>
      </div>
    </form>
  );
}

/** 選択肢を読み込めなかったとき、そのプルダウンの横に出す（一覧の表示は続ける）。 */
function OptionRetry({ onRetry }: { onRetry: () => void }) {
  return (
    <button type="button" onClick={onRetry} className="min-h-11 shrink-0 text-sm text-danger underline underline-offset-4">
      再試行
    </button>
  );
}
