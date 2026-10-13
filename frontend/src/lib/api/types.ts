// API のリクエスト・レスポンスの型（API 仕様書 6 章）。
// null になりうる項目（未分類の category など）は型で null を含め、画面での扱いを強制する。

export type Me = {
  id: number;
  email: string;
  display_name: string | null;
  timezone: string;
  mail_accounts_count: number;
  needs_reauth: boolean;
};

export type Category = {
  id: number;
  name: string;
  parent_id: number | null;
  sort_order: number;
  // 初期作成かどうかの情報のみ。削除の可否には使わない（API-021、Step 5 で変更）
  is_system: boolean;
  // include_counts=true のときだけ含まれる
  transaction_count?: number;
};

export type PaymentMethodKind = "credit_card" | "qr" | "bank" | "cash" | "other";

export type PaymentMethod = {
  id: number;
  name: string;
  kind: PaymentMethodKind;
  issuer: string | null;
  card_last4: string | null;
  is_active: boolean;
};

export type TransactionSource = "email" | "manual";
export type TransactionStatus = "pending" | "confirmed";
export type CategorySource = "rule" | "place_type" | "user" | "default";

export type TransactionSummary = {
  id: number;
  occurred_at: string;
  amount_minor: number;
  currency: string;
  merchant: { id: number; name: string; address: string | null } | null;
  // null は「未分類」
  category: { id: number; name: string } | null;
  payment_method: { id: number; name: string; kind: PaymentMethodKind } | null;
  source: TransactionSource;
  status: TransactionStatus;
  is_possible_duplicate: boolean;
  event_count: number;
};

export type TransactionDetail = TransactionSummary & {
  amount_jpy_minor: number;
  category_source: CategorySource;
  is_user_edited: boolean;
  note: string | null;
  // メールの取込（拡張）までは常に空
  payment_events: unknown[];
};

export type TransactionListMeta = {
  total: number;
  total_amount_minor: number;
};

// API-011 のリクエスト。未指定の項目は送らない
export type CreateTransactionBody = {
  occurred_at: string;
  amount_minor: number;
  merchant_name?: string;
  category_id?: number;
  payment_method_id?: number;
  note?: string;
};

// API-013 のリクエスト。送った項目だけが更新され、null は値を外す
export type UpdateTransactionBody = {
  occurred_at?: string;
  amount_minor?: number;
  category_id?: number | null;
  payment_method_id?: number | null;
  note?: string | null;
  learn_category?: boolean;
};

export type CreateCategoryBody = {
  name: string;
  parent_id?: number | null;
  sort_order?: number;
};

export type UpdateCategoryBody = {
  name?: string;
  parent_id?: number | null;
  sort_order?: number;
};
