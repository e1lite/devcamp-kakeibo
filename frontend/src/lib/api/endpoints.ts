// コアの画面から呼ぶ API（イベント設計 3 章の逆引き表と同じ顔ぶれ）。
// API ID はコメントに残し、仕様書と行き来できるようにする。

import { request } from "@/lib/api/client";
import type {
  Category,
  CreateCategoryBody,
  CreateTransactionBody,
  MailAccount,
  Me,
  PaymentMethod,
  SyncJob,
  TransactionDetail,
  TransactionListMeta,
  TransactionSummary,
  UpdateCategoryBody,
  UpdateTransactionBody,
} from "@/lib/api/types";

/** API-004 現在のユーザ取得。共通レイアウトの 1 か所からだけ呼ぶ（イベント設計 0 章 #1）。 */
export async function getMe(): Promise<Me> {
  return (await request<Me>("/auth/me")).data;
}

/** API-005 ログアウト。401 でもログイン画面へ送らない。 */
export async function logout(): Promise<void> {
  await request<void>("/auth/logout", { method: "POST", skipAuthRedirect: true });
}

/** API-010 取引一覧取得。 */
export function listTransactions(params: URLSearchParams, signal?: AbortSignal) {
  return request<TransactionSummary[], TransactionListMeta>(`/transactions?${params.toString()}`, {
    signal,
  });
}

/** API-012 取引詳細取得。 */
export async function getTransaction(id: number): Promise<TransactionDetail> {
  return (await request<TransactionDetail>(`/transactions/${id}`)).data;
}

/** API-011 取引の手動登録。 */
export async function createTransaction(body: CreateTransactionBody): Promise<TransactionDetail> {
  return (await request<TransactionDetail>("/transactions", { method: "POST", body })).data;
}

/** API-013 取引の修正。送った項目だけが更新される。 */
export async function updateTransaction(id: number, body: UpdateTransactionBody): Promise<TransactionDetail> {
  return (await request<TransactionDetail>(`/transactions/${id}`, { method: "PUT", body })).data;
}

/** API-014 取引の削除（論理削除）。 */
export async function deleteTransaction(id: number): Promise<void> {
  await request<void>(`/transactions/${id}`, { method: "DELETE" });
}

/** API-019 カテゴリ一覧。 */
export async function listCategories({ includeCounts = false } = {}): Promise<Category[]> {
  const query = includeCounts ? "?include_counts=true" : "";
  return (await request<Category[]>(`/categories${query}`)).data;
}

/** API-020 カテゴリ作成。 */
export async function createCategory(body: CreateCategoryBody): Promise<Category> {
  return (await request<Category>("/categories", { method: "POST", body })).data;
}

/** API-021 カテゴリ更新。 */
export async function updateCategory(id: number, body: UpdateCategoryBody): Promise<Category> {
  return (await request<Category>(`/categories/${id}`, { method: "PUT", body })).data;
}

/** API-021 カテゴリ削除。取引は未分類に、子カテゴリは最上位に移る。 */
export async function deleteCategory(id: number): Promise<void> {
  await request<void>(`/categories/${id}`, { method: "DELETE" });
}

/**
 * API-031 決済手段一覧。
 * 取引一覧の絞り込みと取引編集では、利用停止中も含めて取る（イベント設計 0 章 #4）。
 */
export async function listPaymentMethods({ includeInactive = false } = {}): Promise<PaymentMethod[]> {
  const query = includeInactive ? "?include_inactive=true" : "";
  return (await request<PaymentMethod[]>(`/payment-methods${query}`)).data;
}

// --- メール連携（拡張の最小版。docs/mail-import-mvp.md） ---------------------

/** API-006 連携メールアカウント一覧。 */
export async function listMailAccounts(): Promise<MailAccount[]> {
  return (await request<MailAccount[]>("/mail-accounts")).data;
}

/**
 * API-029 メール連携開始。返された URL へブラウザごと遷移する。
 * どのユーザの連携かを特定するため、Bearer トークン付きの POST で URL を受け取る（GET の遷移では送れない）。
 */
export async function startMailLink(redirectUri: string): Promise<{ authorization_url: string; expires_in: number }> {
  return (
    await request<{ authorization_url: string; expires_in: number }>("/mail-accounts/google", {
      method: "POST",
      body: { redirect_uri: redirectUri },
    })
  ).data;
}

/** API-007 連携解除。取り込んだメールと取引は残る。 */
export async function disconnectMailAccount(id: number): Promise<void> {
  await request<void>(`/mail-accounts/${id}`, { method: "DELETE" });
}

/** API-008 手動同期の実行。完了を待たずに返るため、進み具合は API-009 で確かめる。 */
export async function requestSync(id: number): Promise<{ sync_job_id: number }> {
  return (await request<{ sync_job_id: number }>(`/mail-accounts/${id}/sync`, { method: "POST" })).data;
}

/** API-009 同期履歴取得。 */
export async function listSyncJobs(mailAccountId: number, limit = 1): Promise<SyncJob[]> {
  return (await request<SyncJob[]>(`/sync-jobs?mail_account_id=${mailAccountId}&limit=${limit}`)).data;
}
