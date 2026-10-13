// 取引一覧（SCR-010）の絞り込み条件。条件は URL のクエリに持つ（Step 4 判断 5）。
//
// 画面の操作は URL を書き換えるだけにし、API-010 に渡す値はこのファイルの関数で
// URL から組み立てる（イベント設計 SCR-010「URL のクエリと API-010 に渡す値」）。

import { currentMonthToDate } from "@/lib/date";

export const PAGE_SIZE = 20;

export type ListQuery = {
  from?: string;
  to?: string;
  // 整数の文字列か、未分類を表す "none"。不正な値もそのまま API に渡し、400 で知らせる
  categoryId?: string;
  paymentMethodId?: string;
  merchantId?: string;
  q?: string;
  page: number;
};

export type ListFilter = Omit<ListQuery, "page">;

// URL のクエリ名と ListQuery の項目の対応
const PARAMS = [
  ["from", "from"],
  ["to", "to"],
  ["category_id", "categoryId"],
  ["payment_method_id", "paymentMethodId"],
  ["merchant_id", "merchantId"],
  ["q", "q"],
] as const satisfies ReadonlyArray<readonly [string, keyof ListFilter]>;

export function parseListQuery(params: URLSearchParams): ListQuery {
  const query: ListQuery = { page: 1 };
  for (const [name, key] of PARAMS) {
    const value = params.get(name);
    if (value !== null && value !== "") query[key] = value;
  }
  const page = Number(params.get("page"));
  if (Number.isInteger(page) && page >= 1) query.page = page;
  return query;
}

/** URL（/transactions?...）にする。page = 1 と空の条件は省く。 */
export function toListHref(query: ListQuery): string {
  const params = new URLSearchParams();
  for (const [name, key] of PARAMS) {
    const value = query[key];
    if (value) params.set(name, value);
  }
  if (query.page > 1) params.set("page", String(query.page));
  const search = params.toString();
  return search ? `/transactions?${search}` : "/transactions";
}

/**
 * 条件を変えた URL の値。どの条件を変えてもページは 1 に戻す（イベント設計 SCR-010 #2〜#5）。
 * 値に undefined か空文字を渡すと、その条件を外す。
 */
export function withFilter(query: ListQuery, changes: Partial<ListFilter>): ListQuery {
  const next: ListQuery = { ...query, ...changes, page: 1 };
  for (const [, key] of PARAMS) {
    if (!next[key]) delete next[key];
  }
  return next;
}

/**
 * API-010 に渡す期間。
 *
 * - URL に期間があればそれを使う
 * - 無ければ当月 1 日〜今日。ただし店舗で絞り込むとき（merchant_id があるとき）は渡さない＝全期間。
 *   既定の当月のままだと、先月の取引から「この店舗の取引を見る」を押したとき、
 *   押した元の取引すら一覧に出ないため（イベント設計 0 章 #2）
 */
export function effectivePeriod(query: ListQuery, now: Date = new Date()): { from?: string; to?: string } {
  if (query.from || query.to) return { from: query.from, to: query.to };
  if (query.merchantId) return {};
  return currentMonthToDate(now);
}

/** API-010 のクエリ。 */
export function toApiParams(query: ListQuery, now: Date = new Date()): URLSearchParams {
  const params = new URLSearchParams();
  const { from, to } = effectivePeriod(query, now);
  if (from) params.set("from", from);
  if (to) params.set("to", to);
  if (query.categoryId) params.set("category_id", query.categoryId);
  if (query.paymentMethodId) params.set("payment_method_id", query.paymentMethodId);
  if (query.merchantId) params.set("merchant_id", query.merchantId);
  if (query.q) params.set("q", query.q);
  params.set("limit", String(PAGE_SIZE));
  params.set("offset", String((query.page - 1) * PAGE_SIZE));
  return params;
}

/**
 * 条件が付いているか。0 件のときの文言を分ける（Step 5 で見つけた「空の状態が 2 種類」）。
 * 条件が無ければ「まだ取引がありません」、あれば「条件に合う取引はありません」。
 */
export function hasConditions(query: ListQuery): boolean {
  return PARAMS.some(([, key]) => Boolean(query[key]));
}

/** 適用中の条件の数。スマホの「絞り込み（1）」に出す。期間は開始・終了で 1 つと数える。 */
export function appliedCount(query: ListQuery): number {
  let count = query.from || query.to ? 1 : 0;
  for (const key of ["categoryId", "paymentMethodId", "merchantId", "q"] as const) {
    if (query[key]) count += 1;
  }
  return count;
}

/** 開始日が終了日より後なら、欄の下に出す文言（イベント設計 SCR-010 #2）。 */
export function periodError(from: string, to: string): string | null {
  if (from && to && from > to) return "開始日は終了日以前にしてください";
  return null;
}
