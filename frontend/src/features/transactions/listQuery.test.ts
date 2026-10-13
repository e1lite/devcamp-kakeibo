import { describe, expect, it } from "vitest";

import {
  appliedCount,
  effectivePeriod,
  hasConditions,
  parseListQuery,
  periodError,
  toApiParams,
  toListHref,
  withFilter,
} from "@/features/transactions/listQuery";

// 2026-10-06 14:05 JST（UTC では 05:05）
const NOW = new Date("2026-10-06T05:05:00Z");

const parse = (search: string) => parseListQuery(new URLSearchParams(search));

describe("parseListQuery", () => {
  it("URL のクエリを読み、空の値は無視する", () => {
    expect(parse("from=2026-09-01&to=2026-09-30&category_id=none&q=&page=3")).toEqual({
      from: "2026-09-01",
      to: "2026-09-30",
      categoryId: "none",
      page: 3,
    });
  });

  it.each(["0", "-1", "1.5", "abc"])("page=%s は 1 として扱う", (page) => {
    expect(parse(`page=${page}`).page).toBe(1);
  });
});

describe("toListHref", () => {
  it("条件が無ければ /transactions", () => {
    expect(toListHref({ page: 1 })).toBe("/transactions");
  });

  it("page=1 は省き、2 以上なら付ける", () => {
    expect(toListHref({ categoryId: "3", page: 1 })).toBe("/transactions?category_id=3");
    expect(toListHref({ categoryId: "3", page: 2 })).toBe("/transactions?category_id=3&page=2");
  });

  it("読んだ URL に戻せる", () => {
    const href = "/transactions?from=2026-09-01&to=2026-09-30&merchant_id=8&q=%E5%AE%9A%E9%A3%9F&page=2";
    expect(toListHref(parse(href.split("?")[1]))).toBe(href);
  });
});

describe("withFilter", () => {
  it("条件を変えたらページを 1 に戻す（イベント設計 SCR-010 #2〜#5）", () => {
    expect(withFilter({ page: 3 }, { categoryId: "5" })).toEqual({ categoryId: "5", page: 1 });
  });

  it("空の値を渡した条件は外す（店舗の絞り込みの解除 #5）", () => {
    expect(withFilter({ merchantId: "8", q: "定食", page: 2 }, { merchantId: undefined })).toEqual({
      q: "定食",
      page: 1,
    });
  });
});

describe("effectivePeriod / toApiParams", () => {
  it("期間が無ければ当月 1 日〜今日（JST）", () => {
    expect(effectivePeriod({ page: 1 }, NOW)).toEqual({ from: "2026-10-01", to: "2026-10-06" });
  });

  it("JST で月をまたぐ時刻でも、当月は JST で決める", () => {
    // UTC では 9/30 だが JST では 10/1 の 0:30
    expect(effectivePeriod({ page: 1 }, new Date("2026-09-30T15:30:00Z"))).toEqual({
      from: "2026-10-01",
      to: "2026-10-01",
    });
  });

  it("店舗で絞り込むときは期間を渡さない＝全期間（イベント設計 0 章 #2）", () => {
    const params = toApiParams({ merchantId: "8", page: 1 }, NOW);
    expect(params.has("from")).toBe(false);
    expect(params.has("to")).toBe(false);
    expect(params.get("merchant_id")).toBe("8");
  });

  it("店舗で絞り込んでいても、URL に期間があればそれを使う", () => {
    expect(effectivePeriod({ merchantId: "8", from: "2026-01-01", page: 1 }, NOW)).toEqual({
      from: "2026-01-01",
      to: undefined,
    });
  });

  it("未分類は category_id=none のまま渡す", () => {
    expect(toApiParams({ categoryId: "none", page: 1 }, NOW).get("category_id")).toBe("none");
  });

  it("page から limit / offset を計算する", () => {
    const params = toApiParams({ page: 3 }, NOW);
    expect(params.get("limit")).toBe("20");
    expect(params.get("offset")).toBe("40");
  });
});

describe("hasConditions / appliedCount", () => {
  it("条件が無ければ「まだ取引がありません」側", () => {
    expect(hasConditions({ page: 2 })).toBe(false);
  });

  it("期間は開始・終了で 1 つと数える", () => {
    expect(appliedCount({ from: "2026-09-01", to: "2026-09-30", categoryId: "none", q: "a", page: 1 })).toBe(3);
  });
});

describe("periodError", () => {
  it("開始日が終了日より後ならエラー", () => {
    expect(periodError("2026-10-20", "2026-10-06")).toBe("開始日は終了日以前にしてください");
  });

  it("同じ日や片方だけの指定はエラーにしない", () => {
    expect(periodError("2026-10-06", "2026-10-06")).toBeNull();
    expect(periodError("2026-10-06", "")).toBeNull();
  });
});
