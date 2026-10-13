"use client";

import { useRef, useState, type FormEvent } from "react";

import { Button } from "@/components/Button";
import { Dialog, DialogActions } from "@/components/Dialog";
import { ErrorBanner } from "@/components/Feedback";
import { Field, inputClass } from "@/components/Field";
import { CategoryOptions, PaymentMethodOptions } from "@/components/Options";
import {
  isDirty,
  isFuture,
  NOTE_MAX_LENGTH,
  noteLength,
  validateAmount,
  validateForm,
  validateNote,
  validateOccurredAt,
  type TransactionFormErrors,
  type TransactionFormValues,
} from "@/features/transactions/form";
import type { Category, PaymentMethod } from "@/lib/api/types";
import type { ResourceHandle } from "@/lib/useResource";

// 取引の登録（SCR-011）と編集（SCR-013）の共通のフォーム。
// 6 項目を 1 列に、同じ並びで出す。登録と編集、PC とスマホで入力欄の位置が変わらないので、
// 一度覚えれば迷わない（Step 5 のレイアウトの判断 6）。
// 必須で初期値が無い「金額」を先頭に置く（日時・カテゴリ・決済手段には初期値が入るため）。

/** 送信の結果。成功（遷移する）なら何も返さない。 */
export type SubmitOutcome = { fieldErrors?: TransactionFormErrors; message?: string } | void;

type Props = {
  initialValues: TransactionFormValues;
  categories: ResourceHandle<Category[]>;
  paymentMethods: ResourceHandle<PaymentMethod[]>;
  // 選択肢に出す決済手段（登録は利用中のみ、編集は現在の値なら利用停止中も残す）
  selectablePaymentMethods: (methods: PaymentMethod[]) => PaymentMethod[];
  // 編集では店舗名を変更できない（API-013 は店舗の一覧が拡張のため merchant_id しか受け付けない）
  merchantReadOnly: boolean;
  submitLabel: string;
  // 編集では、初期値から何も変えていない間は「保存」を押せない
  requireChange: boolean;
  autoFocusAmount: boolean;
  onSubmit: (values: TransactionFormValues) => Promise<SubmitOutcome>;
  onCancel: () => void;
};

type FieldName = keyof TransactionFormErrors;
const FIELD_ORDER: FieldName[] = ["amount", "occurredAt", "note"];

export function TransactionForm({
  initialValues,
  categories,
  paymentMethods,
  selectablePaymentMethods,
  merchantReadOnly,
  submitLabel,
  requireChange,
  autoFocusAmount,
  onSubmit,
  onCancel,
}: Props) {
  const [values, setValues] = useState(initialValues);
  const [errors, setErrors] = useState<TransactionFormErrors>({});
  const [futureWarning, setFutureWarning] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [confirmingCancel, setConfirmingCancel] = useState(false);
  const topRef = useRef<HTMLDivElement>(null);
  const fieldRefs = useRef<Partial<Record<FieldName, HTMLElement | null>>>({});

  // 初期値があとから決まる項目（登録の決済手段の初期値「現金」は、選択肢を取ってから分かる）。
  // まだ触っていない項目だけを新しい初期値に合わせる
  const [baseline, setBaseline] = useState(initialValues);
  if (initialValues !== baseline) {
    const merged = { ...values };
    for (const key of Object.keys(initialValues) as (keyof TransactionFormValues)[]) {
      if (values[key] === baseline[key]) merged[key] = initialValues[key];
    }
    setBaseline(initialValues);
    setValues(merged);
  }

  // 選択肢を取り直したら、もう無いカテゴリ・決済手段を選んだままにしない
  // （別のタブで削除されて 404 になったとき、選び直してもらうため）
  const categoryIds = categories.state.status === "success" ? new Set(categories.state.data.map((c) => String(c.id))) : null;
  const methods =
    paymentMethods.state.status === "success" ? selectablePaymentMethods(paymentMethods.state.data) : null;
  if (categoryIds && values.categoryId && !categoryIds.has(values.categoryId)) {
    setValues({ ...values, categoryId: "" });
  }
  if (methods && values.paymentMethodId && !methods.some((m) => String(m.id) === values.paymentMethodId)) {
    setValues({ ...values, paymentMethodId: "" });
  }

  const set = (key: keyof TransactionFormValues) => (value: string) => setValues((v) => ({ ...v, [key]: value }));

  // 欄から離れたときに検証する（SCR-011 #2）
  const validateOnBlur = (name: FieldName) => () => {
    const validators = { amount: validateAmount, occurredAt: validateOccurredAt, note: validateNote };
    setErrors((e) => ({ ...e, [name]: validators[name](values[name]) }));
    if (name === "occurredAt") setFutureWarning(isFuture(values));
  };

  const optionsReady = categories.state.status === "success" && paymentMethods.state.status === "success";
  const dirty = isDirty(baseline, values);
  const canSubmit = optionsReady && !submitting && (!requireChange || dirty);

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    if (!canSubmit) return;

    // 送る前に検証をすべて行い、エラーがあれば API を呼ばない。最初のエラー欄へフォーカスを移す
    const found = validateForm(values);
    setErrors(found);
    const firstError = FIELD_ORDER.find((name) => found[name]);
    if (firstError) {
      fieldRefs.current[firstError]?.focus();
      return;
    }

    setSubmitting(true);
    setMessage(null);
    const outcome = await onSubmit(values);
    if (!outcome) return; // 成功。遷移するまでボタンは押せないままにする（二重送信を防ぐ）

    setSubmitting(false);
    if (outcome.fieldErrors) {
      setErrors((e) => ({ ...e, ...outcome.fieldErrors }));
      const first = FIELD_ORDER.find((name) => outcome.fieldErrors?.[name]);
      if (first) fieldRefs.current[first]?.focus();
    }
    if (outcome.message) {
      // 特定の欄に結びつかないエラーは画面上部に出し、先頭までスクロールする（SCR-011 #9）
      setMessage(outcome.message);
      topRef.current?.scrollIntoView({ block: "start" });
    }
  };

  // キャンセル（SCR-011 #4 / SCR-013 #4）。入力途中なら確認してから戻る
  const handleCancel = () => {
    if (dirty) setConfirmingCancel(true);
    else onCancel();
  };

  return (
    <form onSubmit={handleSubmit} noValidate className="flex flex-col gap-5">
      <div ref={topRef} className="scroll-mt-20">
        {message && <ErrorBanner>{message}</ErrorBanner>}
      </div>

      <Field label="金額" required error={errors.amount}>
        {(control) => (
          <div className="relative">
            <span aria-hidden="true" className="pointer-events-none absolute inset-y-0 left-3 flex items-center text-muted">
              ¥
            </span>
            <input
              {...control}
              ref={(el) => {
                fieldRefs.current.amount = el;
              }}
              type="text"
              // スマホでは数字のキーボードを出す
              inputMode="numeric"
              autoComplete="off"
              placeholder="例: 1200"
              autoFocus={autoFocusAmount}
              className={`${inputClass} amount pl-8`}
              value={values.amount}
              onChange={(e) => set("amount")(e.target.value)}
              onBlur={validateOnBlur("amount")}
            />
          </div>
        )}
      </Field>

      <Field
        label="日時"
        required
        error={errors.occurredAt}
        warning={futureWarning ? "未来の日時です。このまま登録できます" : undefined}
      >
        {(control) => (
          <input
            {...control}
            ref={(el) => {
              fieldRefs.current.occurredAt = el;
            }}
            type="datetime-local"
            className={inputClass}
            value={values.occurredAt}
            onChange={(e) => set("occurredAt")(e.target.value)}
            onBlur={validateOnBlur("occurredAt")}
          />
        )}
      </Field>

      {merchantReadOnly ? (
        // 枠の無い文字で出し、入力欄と見分けられるようにする（SCR-013 #1）
        <div className="flex flex-col gap-1">
          <span className="text-sm font-bold">店舗名</span>
          <p className="min-h-11 py-2">{values.merchantName || <span className="text-muted">店舗名なし</span>}</p>
          <p className="text-xs text-muted">店舗名は変更できません</p>
        </div>
      ) : (
        <Field label="店舗名">
          {(control) => (
            <input
              {...control}
              type="text"
              placeholder="例: 近所の定食屋"
              className={inputClass}
              value={values.merchantName}
              onChange={(e) => set("merchantName")(e.target.value)}
            />
          )}
        </Field>
      )}

      <Field label="カテゴリ" hint={<OptionStatus handle={categories} />}>
        {(control) => (
          <select
            {...control}
            className={inputClass}
            value={values.categoryId}
            disabled={categories.state.status !== "success"}
            onChange={(e) => set("categoryId")(e.target.value)}
          >
            {categories.state.status === "loading" ? (
              <option value={values.categoryId}>読み込み中…</option>
            ) : (
              <>
                {/* 未分類は category_id を送らない（編集では null） */}
                <option value="">未分類</option>
                {categories.state.status === "success" && <CategoryOptions categories={categories.state.data} />}
              </>
            )}
          </select>
        )}
      </Field>

      <Field label="決済手段" hint={<OptionStatus handle={paymentMethods} />}>
        {(control) => (
          <select
            {...control}
            className={inputClass}
            value={values.paymentMethodId}
            disabled={paymentMethods.state.status !== "success"}
            onChange={(e) => set("paymentMethodId")(e.target.value)}
          >
            {paymentMethods.state.status === "loading" ? (
              <option value={values.paymentMethodId}>読み込み中…</option>
            ) : (
              <>
                <option value="">なし</option>
                {methods && <PaymentMethodOptions methods={methods} />}
              </>
            )}
          </select>
        )}
      </Field>

      <Field
        label="メモ"
        error={errors.note}
        hint={
          <span className="amount" aria-live="polite">
            {noteLength(values.note)} / {NOTE_MAX_LENGTH}
          </span>
        }
      >
        {(control) => (
          <textarea
            {...control}
            ref={(el) => {
              fieldRefs.current.note = el;
            }}
            rows={3}
            className={`${inputClass} py-2`}
            value={values.note}
            onChange={(e) => set("note")(e.target.value)}
            onBlur={validateOnBlur("note")}
          />
        )}
      </Field>

      {/* PC は左に「キャンセル」、右に主ボタン。スマホは主ボタンを上にして縦に並べる */}
      <div className="flex flex-col-reverse gap-3 md:flex-row md:justify-between">
        <Button variant="secondary" onClick={handleCancel} disabled={submitting}>
          キャンセル
        </Button>
        <Button type="submit" variant="primary" disabled={!canSubmit} className="md:min-w-32">
          {submitting ? "送信中…" : submitLabel}
        </Button>
      </div>

      <Dialog open={confirmingCancel} title="入力内容を破棄しますか" onClose={() => setConfirmingCancel(false)}>
        <p className="text-sm">入力した金額や店舗名は保存されません。</p>
        <DialogActions>
          {/* 初期フォーカスは「入力を続ける」（Enter の押し間違いで捨てないため） */}
          <Button variant="secondary" data-autofocus onClick={() => setConfirmingCancel(false)}>
            入力を続ける
          </Button>
          <Button variant="danger" onClick={onCancel}>
            破棄する
          </Button>
        </DialogActions>
      </Dialog>
    </form>
  );
}

/** 選択肢を読み込めなかったとき、欄の下に「再試行」を出す。 */
function OptionStatus<T>({ handle }: { handle: ResourceHandle<T> }) {
  if (handle.state.status !== "error") return null;
  return (
    <span className="text-danger">
      選択肢を読み込めませんでした。{" "}
      <button type="button" onClick={handle.retry} className="underline underline-offset-4">
        再試行
      </button>
    </span>
  );
}
