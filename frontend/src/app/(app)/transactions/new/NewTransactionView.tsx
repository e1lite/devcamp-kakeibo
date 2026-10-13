"use client";

import { useCallback, useMemo } from "react";

import { Page, useApp } from "@/components/app/AppShell";
import { TransactionForm } from "@/features/transactions/components/TransactionForm";
import { toCreateBody, type TransactionFormValues } from "@/features/transactions/form";
import { submitOutcome } from "@/features/transactions/submitErrors";
import { createTransaction, listCategories, listPaymentMethods } from "@/lib/api/endpoints";
import type { PaymentMethod } from "@/lib/api/types";
import { currentMonthToDate, monthOf, nowLocalInput } from "@/lib/date";
import { useResource } from "@/lib/useResource";

const loadCategories = () => listCategories();
// 登録では利用中の決済手段だけを選ばせる
const loadPaymentMethods = () => listPaymentMethods();
const onlyActive = (methods: PaymentMethod[]) => methods.filter((m) => m.is_active);

/**
 * SCR-011 取引登録。現金など、メールが届かない支出を手入力する。
 * レシートを見ながら、金額を入れてすぐ登録できることを優先する。
 */
export function NewTransactionView() {
  const { flash, back, replace } = useApp();
  const categories = useResource(loadCategories);
  const paymentMethods = useResource(loadPaymentMethods);

  // 初期値は 日時 = 現在時刻、カテゴリ = 未分類、決済手段 = 現金（kind = cash）。
  // 現金の ID は選択肢を取ってから分かるため、そろったら決済手段の初期値を入れる
  const cashId =
    paymentMethods.state.status === "success"
      ? (paymentMethods.state.data.find((m) => m.kind === "cash" && m.is_active)?.id ?? null)
      : null;
  const initialValues = useMemo<TransactionFormValues>(
    () => ({
      amount: "",
      occurredAt: nowLocalInput(),
      merchantName: "",
      categoryId: "",
      paymentMethodId: cashId === null ? "" : String(cashId),
      note: "",
    }),
    [cashId],
  );

  const { retry: retryCategories } = categories;
  const { retry: retryPaymentMethods } = paymentMethods;

  const onSubmit = useCallback(
    async (values: TransactionFormValues) => {
      try {
        const created = await createTransaction(toCreateBody(values));
        // 登録した取引の月を期間にして一覧を開く。先月のレシートを入れたとき、
        // 当月の一覧に戻っても登録できたか分からないため（イベント設計 0 章 #5）
        const month = monthOf(created.occurred_at);
        const href =
          month.from === currentMonthToDate().from ? "/transactions" : `/transactions?from=${month.from}&to=${month.to}`;
        flash("登録しました");
        replace(href);
      } catch (error) {
        return submitOutcome(error, "登録", {
          onCategoryMissing: retryCategories,
          onPaymentMethodMissing: retryPaymentMethods,
        });
      }
    },
    [flash, replace, retryCategories, retryPaymentMethods],
  );

  return (
    <Page width="form">
      <div className="flex flex-col gap-1">
        <h1 className="text-2xl font-bold">取引の登録</h1>
        <p className="text-sm text-muted">現金など、通知メールが届かない支出を入力します。</p>
      </div>
      <TransactionForm
        initialValues={initialValues}
        categories={categories}
        paymentMethods={paymentMethods}
        selectablePaymentMethods={onlyActive}
        merchantReadOnly={false}
        submitLabel="登録"
        requireChange={false}
        autoFocusAmount
        onSubmit={onSubmit}
        // 破棄したら一覧へ戻る（履歴を戻る。履歴が無ければ取引一覧）
        onCancel={() => back("/transactions")}
      />
    </Page>
  );
}
