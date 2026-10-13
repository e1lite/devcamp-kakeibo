"use client";

import { useCallback, useMemo, useState } from "react";

import { parseTransactionId } from "@/app/(app)/transactions/[id]/TransactionDetailView";
import { Page, useApp } from "@/components/app/AppShell";
import { ErrorBox, NOT_FOUND_TRANSACTION } from "@/components/ErrorScreen";
import { LoadError } from "@/components/Feedback";
import { TransactionForm } from "@/features/transactions/components/TransactionForm";
import { toUpdateBody, valuesFromTransaction, type TransactionFormValues } from "@/features/transactions/form";
import { submitOutcome } from "@/features/transactions/submitErrors";
import { ApiError } from "@/lib/api/client";
import { getTransaction, listCategories, listPaymentMethods, updateTransaction } from "@/lib/api/endpoints";
import type { PaymentMethod } from "@/lib/api/types";
import { useResource } from "@/lib/useResource";

const loadCategories = () => listCategories();
// 利用停止中も含めて取り、この取引の現在の値のときだけ選択肢に残す（イベント設計 0 章 #4）
const loadPaymentMethods = () => listPaymentMethods({ includeInactive: true });

/** SCR-013 取引編集。入力欄の並びは取引登録（SCR-011）と同じにする。 */
export function EditTransactionView({ id }: { id: string }) {
  const transactionId = parseTransactionId(id);
  if (transactionId === null) {
    return (
      <Page width="form">
        <ErrorBox {...NOT_FOUND_TRANSACTION} />
      </Page>
    );
  }
  return <EditTransactionContent id={transactionId} />;
}

function EditTransactionContent({ id }: { id: number }) {
  const { flash, back } = useApp();
  const loadTransaction = useCallback(() => getTransaction(id), [id]);
  const transaction = useResource(loadTransaction);
  const categories = useResource(loadCategories);
  const paymentMethods = useResource(loadPaymentMethods);
  // 保存しようとしたら、編集中に別のタブで削除されていた
  const [missing, setMissing] = useState(false);

  const txn = transaction.state.status === "success" ? transaction.state.data : null;
  const initialValues = useMemo(() => (txn ? valuesFromTransaction(txn) : null), [txn]);

  // 利用停止中の決済手段は、この取引の現在の値のときだけ残す
  const currentPaymentMethodId = initialValues?.paymentMethodId ?? "";
  const selectablePaymentMethods = useCallback(
    (methods: PaymentMethod[]) => methods.filter((m) => m.is_active || String(m.id) === currentPaymentMethodId),
    [currentPaymentMethodId],
  );

  const detailHref = `/transactions/${id}`;
  const { retry: retryCategories } = categories;
  const { retry: retryPaymentMethods } = paymentMethods;

  const onSubmit = useCallback(
    async (values: TransactionFormValues) => {
      if (!initialValues) return;
      try {
        // 初期値から変わった項目だけを送る（SCR-013 #3）
        await updateTransaction(id, toUpdateBody(initialValues, values));
        flash("保存しました");
        // 取引詳細へ戻る。履歴を戻るので、ブラウザの「戻る」で編集画面に戻らない
        back(detailHref);
      } catch (error) {
        return submitOutcome(error, "保存", {
          onCategoryMissing: retryCategories,
          onPaymentMethodMissing: retryPaymentMethods,
          onTransactionMissing: () => setMissing(true),
        });
      }
    },
    [id, initialValues, flash, back, detailHref, retryCategories, retryPaymentMethods],
  );

  const transactionError = transaction.state.status === "error" ? transaction.state.error : null;
  if (missing || (transactionError instanceof ApiError && transactionError.status === 404)) {
    return (
      <Page width="form">
        <ErrorBox {...NOT_FOUND_TRANSACTION} />
      </Page>
    );
  }

  return (
    <Page width="form">
      <h1 className="text-2xl font-bold">取引の編集</h1>

      {transactionError ? (
        <LoadError message="取引を読み込めませんでした。" onRetry={transaction.retry} />
      ) : !initialValues ? (
        // 入力欄の形のスケルトン。空の欄を先に見せると、値が無い取引だと誤解されるため
        <div className="flex flex-col gap-6">
          {Array.from({ length: 6 }, (_, i) => (
            <div key={i} className="flex flex-col gap-2">
              <div className="h-4 w-16 animate-pulse rounded bg-skeleton" />
              <div className="h-11 animate-pulse rounded-md bg-skeleton" />
            </div>
          ))}
        </div>
      ) : (
        <TransactionForm
          key={id}
          initialValues={initialValues}
          categories={categories}
          paymentMethods={paymentMethods}
          selectablePaymentMethods={selectablePaymentMethods}
          merchantReadOnly
          submitLabel="保存"
          requireChange
          autoFocusAmount={false}
          onSubmit={onSubmit}
          onCancel={() => back(detailHref)}
        />
      )}
    </Page>
  );
}
