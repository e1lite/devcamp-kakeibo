import { operationErrorMessage } from "@/components/Feedback";
import type { SubmitOutcome } from "@/features/transactions/components/TransactionForm";
import { ApiError, isRetryable } from "@/lib/api/client";

// 取引の登録（API-011）・修正（API-013）のエラーを、画面での出し方に対応づける
// （イベント設計 SCR-011・SCR-013 の「API の結果」の表）。

type Handlers = {
  // 選んだカテゴリ・決済手段が別のタブで削除されていた。選択肢を取り直す
  onCategoryMissing: () => void;
  onPaymentMethodMissing: () => void;
  // 編集中に取引が削除されていた（API-013 の 404 TRANSACTION_NOT_FOUND）
  onTransactionMissing?: () => void;
};

export function submitOutcome(error: unknown, button: string, handlers: Handlers): SubmitOutcome {
  if (isRetryable(error)) {
    // 入力内容を保ったまま、ボタンをもう一度押せる状態に戻す（イベント設計 1.2）
    return { message: operationErrorMessage(button) };
  }
  if (!(error instanceof ApiError)) throw error;

  switch (error.code) {
    case "INVALID_AMOUNT":
      return { fieldErrors: { amount: "1 以上の整数で入力してください" } };
    case "VALIDATION_ERROR":
      return { message: "入力内容を確認してください。" };
    case "CATEGORY_NOT_FOUND":
      handlers.onCategoryMissing();
      return { message: "選んだカテゴリが見つかりません。選び直してください。" };
    case "PAYMENT_METHOD_NOT_FOUND":
      handlers.onPaymentMethodMissing();
      return { message: "選んだ決済手段が見つかりません。選び直してください。" };
    case "TRANSACTION_NOT_FOUND":
      if (handlers.onTransactionMissing) {
        handlers.onTransactionMissing();
        return { message: "取引が見つかりません。" };
      }
      break;
  }
  return { message: `${button}できませんでした。入力内容を確認してください。` };
}
