import { describe, expect, it } from "vitest";

import {
  isDirty,
  isFuture,
  noteLength,
  parseAmount,
  toCreateBody,
  toUpdateBody,
  validateAmount,
  validateForm,
  valuesFromTransaction,
  type TransactionFormValues,
} from "@/features/transactions/form";
import type { TransactionDetail } from "@/lib/api/types";

const base: TransactionFormValues = {
  amount: "1200",
  occurredAt: "2026-09-10T19:20",
  merchantName: "",
  categoryId: "",
  paymentMethodId: "1",
  note: "",
};

describe("parseAmount / validateAmount", () => {
  it.each([
    ["1200", 1200],
    ["１２００", 1200],
    ["12,000", 12000],
    [" 580 ", 580],
  ])("%s を %d と読む", (input, expected) => {
    expect(parseAmount(input)).toBe(expected);
  });

  it.each(["0", "12.5", "-100", "abc", "1e3"])("%s は 1 以上の整数ではない", (input) => {
    expect(parseAmount(input)).toBeNull();
    expect(validateAmount(input)).toBe("1 以上の整数で入力してください");
  });

  it("空なら入力を促す", () => {
    expect(validateAmount("")).toBe("金額を入力してください");
  });
});

describe("validateForm", () => {
  it("メモはサロゲートペアを 1 文字と数え、500 文字まで", () => {
    expect(noteLength("🍙🍙")).toBe(2);
    expect(validateForm({ ...base, note: "あ".repeat(500) })).toEqual({});
    expect(validateForm({ ...base, note: "あ".repeat(501) }).note).toBe("500 文字以内で入力してください");
  });

  it("日時が空ならエラー", () => {
    expect(validateForm({ ...base, occurredAt: "" }).occurredAt).toBe("日時を入力してください");
  });
});

describe("isFuture", () => {
  it("現在より後の日時は警告の対象", () => {
    const now = new Date("2026-10-06T05:05:00Z"); // 14:05 JST
    expect(isFuture({ ...base, occurredAt: "2026-10-06T14:06" }, now)).toBe(true);
    expect(isFuture({ ...base, occurredAt: "2026-10-06T14:05" }, now)).toBe(false);
  });
});

describe("toCreateBody（イベント設計 SCR-011 の送り方）", () => {
  it("空の任意項目は送らず、日時に +09:00 を付ける", () => {
    expect(toCreateBody({ ...base, paymentMethodId: "" })).toEqual({
      occurred_at: "2026-09-10T19:20:00+09:00",
      amount_minor: 1200,
    });
  });

  it("店舗名は前後の空白を除いて送る", () => {
    expect(
      toCreateBody({ ...base, merchantName: "  近所の定食屋 ", categoryId: "3", note: "ランチ" }),
    ).toEqual({
      occurred_at: "2026-09-10T19:20:00+09:00",
      amount_minor: 1200,
      merchant_name: "近所の定食屋",
      category_id: 3,
      payment_method_id: 1,
      note: "ランチ",
    });
  });

  it("空白だけのメモは送らない", () => {
    expect(toCreateBody({ ...base, note: "   " })).not.toHaveProperty("note");
  });
});

describe("toUpdateBody（イベント設計 SCR-013 の送る値）", () => {
  const initial: TransactionFormValues = { ...base, categoryId: "3", note: "メモ" };

  it("何も変えていなければ空", () => {
    expect(toUpdateBody(initial, { ...initial })).toEqual({});
    expect(isDirty(initial, { ...initial })).toBe(false);
  });

  it("変えた項目だけを送る", () => {
    expect(toUpdateBody(initial, { ...initial, amount: "1500" })).toEqual({ amount_minor: 1500 });
  });

  it("カテゴリを変えたら learn_category: false も送る（0 章 #3）", () => {
    expect(toUpdateBody(initial, { ...initial, categoryId: "5" })).toEqual({
      category_id: 5,
      learn_category: false,
    });
  });

  it("未分類に戻す・決済手段を外す・メモを空にするときは null", () => {
    expect(toUpdateBody(initial, { ...initial, categoryId: "", paymentMethodId: "", note: "" })).toEqual({
      category_id: null,
      learn_category: false,
      payment_method_id: null,
      note: null,
    });
  });

  it("金額の書き方だけが違う（1200 → １２００）なら送らない", () => {
    expect(toUpdateBody(initial, { ...initial, amount: "１２００" })).toEqual({});
  });
});

describe("valuesFromTransaction", () => {
  it("取引の値を JST の入力欄の値にする", () => {
    const txn = {
      id: 1,
      occurred_at: "2026-09-10T10:20:30Z",
      amount_minor: 580,
      currency: "JPY",
      merchant: null,
      category: null,
      payment_method: { id: 1, name: "現金", kind: "cash" },
      source: "manual",
      status: "confirmed",
      is_possible_duplicate: false,
      event_count: 0,
      amount_jpy_minor: 580,
      category_source: "default",
      is_user_edited: true,
      note: null,
      payment_events: [],
    } satisfies TransactionDetail;

    expect(valuesFromTransaction(txn)).toEqual({
      amount: "580",
      occurredAt: "2026-09-10T19:20",
      merchantName: "",
      categoryId: "",
      paymentMethodId: "1",
      note: "",
    });
  });
});
