// 取引の登録（SCR-011）・編集（SCR-013）の入力値。検証と、API に送る値の組み立て。
//
// 入力欄の値はすべて文字列で持ち、送るときにこのファイルの関数で API の形にする。
// 「何を送るか」と同じく「何を送らないか」も決める（Step 5 のまとめ「既定値も画面の設計の一部」）。

import type { CreateTransactionBody, TransactionDetail, UpdateTransactionBody } from "@/lib/api/types";
import { isFutureLocalInput, isLocalInput, isoToLocalInput, localInputToIso } from "@/lib/date";

export const NOTE_MAX_LENGTH = 500;

export type TransactionFormValues = {
  amount: string;
  // datetime-local の値（JST の YYYY-MM-DDTHH:mm）
  occurredAt: string;
  merchantName: string;
  // "" は未分類
  categoryId: string;
  // "" は未選択
  paymentMethodId: string;
  note: string;
};

export type TransactionFormErrors = Partial<Record<"amount" | "occurredAt" | "note", string>>;

/** 全角の数字と桁区切りを受け付ける（レシートを見ながら打つと混ざりやすいため）。 */
function normalizeAmount(value: string): string {
  return value
    .trim()
    .replace(/[０-９]/g, (c) => String.fromCharCode(c.charCodeAt(0) - 0xfee0))
    .replace(/[,，]/g, "");
}

/** 金額の入力を円の整数にする。1 以上の整数でなければ null。 */
export function parseAmount(value: string): number | null {
  const normalized = normalizeAmount(value);
  if (!/^\d+$/.test(normalized)) return null;
  const amount = Number(normalized);
  return Number.isSafeInteger(amount) && amount >= 1 ? amount : null;
}

/** 文字数。サーバの 500 文字の判定（Go の rune 数）と合わせ、サロゲートペアを 1 文字と数える。 */
export function noteLength(note: string): number {
  return [...note].length;
}

export function validateAmount(value: string): string | undefined {
  if (normalizeAmount(value) === "") return "金額を入力してください";
  if (parseAmount(value) === null) return "1 以上の整数で入力してください";
  return undefined;
}

export function validateOccurredAt(value: string): string | undefined {
  return isLocalInput(value) ? undefined : "日時を入力してください";
}

export function validateNote(value: string): string | undefined {
  return noteLength(value) > NOTE_MAX_LENGTH ? `${NOTE_MAX_LENGTH} 文字以内で入力してください` : undefined;
}

export function validateForm(values: TransactionFormValues): TransactionFormErrors {
  const errors: TransactionFormErrors = {};
  const amount = validateAmount(values.amount);
  if (amount) errors.amount = amount;
  const occurredAt = validateOccurredAt(values.occurredAt);
  if (occurredAt) errors.occurredAt = occurredAt;
  const note = validateNote(values.note);
  if (note) errors.note = note;
  return errors;
}

/** 未来の日時なら警告だけ出す。予約の支払いなどもあるため、登録は止めない（イベント設計 SCR-011 #2）。 */
export function isFuture(values: TransactionFormValues, now: Date = new Date()): boolean {
  return isLocalInput(values.occurredAt) && isFutureLocalInput(values.occurredAt, now);
}

/**
 * API-011 に送る値（イベント設計 SCR-011 の送り方の表）。
 * 店舗名・カテゴリ・決済手段・メモは、空なら送らない。通貨は送らない（サーバの既定の JPY）。
 * validateForm でエラーが無いことを確かめてから呼ぶこと。
 */
export function toCreateBody(values: TransactionFormValues): CreateTransactionBody {
  const body: CreateTransactionBody = {
    occurred_at: localInputToIso(values.occurredAt),
    amount_minor: parseAmount(values.amount)!,
  };
  const merchantName = values.merchantName.trim();
  if (merchantName) body.merchant_name = merchantName;
  if (values.categoryId) body.category_id = Number(values.categoryId);
  if (values.paymentMethodId) body.payment_method_id = Number(values.paymentMethodId);
  if (values.note.trim()) body.note = values.note;
  return body;
}

/** 取引の現在の値を、編集フォームの初期値にする（SCR-013 #1）。 */
export function valuesFromTransaction(txn: TransactionDetail): TransactionFormValues {
  return {
    amount: String(txn.amount_minor),
    occurredAt: isoToLocalInput(txn.occurred_at),
    merchantName: txn.merchant?.name ?? "",
    categoryId: txn.category ? String(txn.category.id) : "",
    paymentMethodId: txn.payment_method ? String(txn.payment_method.id) : "",
    note: txn.note ?? "",
  };
}

/**
 * API-013 に送る値。初期値から変わった項目だけを送る（イベント設計 SCR-013 #3）。
 *
 * - 未分類に戻した・決済手段を外した・メモを空にした → null
 * - カテゴリを変えた → learn_category: false も送る。省略すると true になり、
 *   学習を実装した時点で画面に何の表示も無いまま学習が始まるため（イベント設計 0 章 #3）
 * - 店舗名は変更できないため送らない
 */
export function toUpdateBody(initial: TransactionFormValues, values: TransactionFormValues): UpdateTransactionBody {
  const body: UpdateTransactionBody = {};

  if (parseAmount(values.amount) !== parseAmount(initial.amount)) {
    body.amount_minor = parseAmount(values.amount)!;
  }
  if (values.occurredAt !== initial.occurredAt) {
    body.occurred_at = localInputToIso(values.occurredAt);
  }
  if (values.categoryId !== initial.categoryId) {
    body.category_id = values.categoryId ? Number(values.categoryId) : null;
    body.learn_category = false;
  }
  if (values.paymentMethodId !== initial.paymentMethodId) {
    body.payment_method_id = values.paymentMethodId ? Number(values.paymentMethodId) : null;
  }
  if (normalizeNote(values.note) !== normalizeNote(initial.note)) {
    body.note = normalizeNote(values.note);
  }
  return body;
}

// 空白だけのメモは「メモなし」として扱う
function normalizeNote(note: string): string | null {
  return note.trim() ? note : null;
}

/** 入力途中か（キャンセルの確認を出すか、編集で「保存」を押せるか）。 */
export function isDirty(initial: TransactionFormValues, values: TransactionFormValues): boolean {
  return (Object.keys(initial) as (keyof TransactionFormValues)[]).some((key) => initial[key] !== values[key]);
}
