"use client";

import Link from "next/link";
import { useCallback, useState } from "react";

import { Page, useApp } from "@/components/app/AppShell";
import { Button, LinkButton } from "@/components/Button";
import { Dialog, DialogActions } from "@/components/Dialog";
import { ErrorBox, NOT_FOUND_TRANSACTION } from "@/components/ErrorScreen";
import { ErrorBanner, LoadError, Skeleton } from "@/components/Feedback";
import { ApiError, isRetryable } from "@/lib/api/client";
import { deleteTransaction, getTransaction } from "@/lib/api/endpoints";
import type { TransactionDetail } from "@/lib/api/types";
import { formatDateTimeWithWeekday, formatShortDateTime } from "@/lib/date";
import { formatYen } from "@/lib/format";
import { useResource } from "@/lib/useResource";

/** URL の id が数字でなければ API を呼ばず、取引が見つからない表示にする（SCR-012 #1）。 */
export function parseTransactionId(id: string): number | null {
  return /^\d+$/.test(id) ? Number(id) : null;
}

/** SCR-012 取引詳細。自動で分類された取引が正しいかを判断できる情報を並べる。 */
export function TransactionDetailView({ id }: { id: string }) {
  const transactionId = parseTransactionId(id);
  if (transactionId === null) {
    return (
      <Page width="detail">
        <ErrorBox {...NOT_FOUND_TRANSACTION} />
      </Page>
    );
  }
  return <TransactionDetailContent id={transactionId} />;
}

function TransactionDetailContent({ id }: { id: number }) {
  const { back } = useApp();
  const load = useCallback(() => getTransaction(id), [id]);
  const transaction = useResource(load);
  const [deleting, setDeleting] = useState(false);

  const { state } = transaction;

  if (state.status === "error") {
    // 404 は存在しない・削除済みに加え、他のユーザの取引も同じ表示にする（Step 4 のフィードバック）。
    // URL は変えない
    if (state.error instanceof ApiError && state.error.status === 404) {
      return (
        <Page width="detail">
          <ErrorBox {...NOT_FOUND_TRANSACTION} />
        </Page>
      );
    }
    return (
      <Page width="detail">
        <LoadError message="取引を読み込めませんでした。" onRetry={transaction.retry} />
      </Page>
    );
  }

  return (
    <Page width="detail">
      {/* 履歴を戻るので、一覧の絞り込み条件が保たれる（SCR-012 #7） */}
      <button
        type="button"
        onClick={() => back("/transactions")}
        className="inline-flex min-h-11 items-center self-start text-sm underline underline-offset-4"
      >
        ‹ 戻る
      </button>

      {state.status === "loading" ? (
        // 「編集」「削除」はまだ出さない
        <div className="flex flex-col gap-4 rounded-md border border-line bg-surface p-5">
          <Skeleton rows={3} />
          <Skeleton rows={5} />
        </div>
      ) : (
        <>
          <Summary txn={state.data} />
          <Details txn={state.data} />
          <section aria-label="削除" className="flex flex-col items-start gap-3 border-t border-line pt-6">
            <p className="text-sm text-muted">削除した取引は、一覧と合計から外れます。</p>
            <Button variant="danger" onClick={() => setDeleting(true)}>
              削除
            </Button>
          </section>
          <DeleteDialog txn={state.data} open={deleting} onClose={() => setDeleting(false)} />
        </>
      )}
    </Page>
  );
}

/** A 概要。何の支出か・いくらか・いつかが最初に目に入るようにする。 */
function Summary({ txn }: { txn: TransactionDetail }) {
  return (
    <section className="flex flex-col gap-4 rounded-md border border-line bg-surface p-5">
      <div className="flex items-start justify-between gap-4">
        <h1 className="text-xl font-bold break-all">
          {txn.merchant ? txn.merchant.name : <span className="text-muted">店舗名なし</span>}
        </h1>
        {/* 主ボタン。PC は概要の右、スマホは概要の下の全幅 */}
        <LinkButton href={`/transactions/${txn.id}/edit`} variant="primary" className="max-md:hidden">
          編集
        </LinkButton>
      </div>
      {/* 金額は画面で最も大きな文字にする */}
      <p className="amount text-4xl font-bold">{formatYen(txn.amount_minor)}</p>
      {txn.status === "pending" && (
        <p className="self-start rounded-sm border border-warn px-2 py-0.5 text-sm text-warn">速報（確定前の金額）</p>
      )}
      <p className="amount text-sm text-muted">{formatDateTimeWithWeekday(txn.occurred_at)}</p>
      <LinkButton href={`/transactions/${txn.id}/edit`} variant="primary" className="w-full md:hidden">
        編集
      </LinkButton>
    </section>
  );
}

/** B 詳細。項目名と値の 2 列（スマホは 1 列）。値が空の項目も「なし」と出し、行の位置を変えない。 */
function Details({ txn }: { txn: TransactionDetail }) {
  return (
    <dl className="grid grid-cols-1 gap-x-6 gap-y-4 rounded-md border border-line bg-surface p-5 md:grid-cols-[8rem_1fr]">
      <dt className="text-sm font-bold text-muted">カテゴリ</dt>
      <dd className="flex items-center gap-2">
        {txn.category ? (
          <>
            {txn.category.name}
            {/* 分類の由来。手入力でカテゴリを選ばなかった取引は default になるため、
                user 以外を「自動分類」とすると未分類に「自動分類」と出てしまう。未分類なら印を付けない */}
            <span className="rounded-sm border border-line px-1.5 text-xs text-muted">
              {txn.category_source === "user" ? "手動" : "自動分類"}
            </span>
          </>
        ) : (
          "未分類"
        )}
      </dd>

      <dt className="text-sm font-bold text-muted">決済手段</dt>
      <dd>{txn.payment_method?.name ?? "なし"}</dd>

      {/* 店舗の行は、店舗があるときだけ出す */}
      {txn.merchant && (
        <>
          <dt className="text-sm font-bold text-muted">店舗</dt>
          <dd className="flex flex-col items-start gap-1">
            {txn.merchant.address && <span>{txn.merchant.address}</span>}
            {/* 期間は付けない＝全期間（イベント設計 0 章 #2） */}
            <Link
              href={`/transactions?merchant_id=${txn.merchant.id}`}
              className="inline-flex min-h-11 items-center text-sm underline underline-offset-4"
            >
              この店舗の取引を見る ›
            </Link>
          </dd>
        </>
      )}

      <dt className="text-sm font-bold text-muted">取込元</dt>
      <dd>{txn.source === "email" ? "メール" : "手入力"}</dd>

      <dt className="text-sm font-bold text-muted">メモ</dt>
      <dd className="break-words whitespace-pre-wrap">{txn.note ?? "なし"}</dd>
    </dl>
  );
}

/** 削除の確認（SCR-012 #3〜#5）。どの取引かを日時・店舗・金額で示す。 */
function DeleteDialog({ txn, open, onClose }: { txn: TransactionDetail; open: boolean; onClose: () => void }) {
  const { flash, back } = useApp();
  const [submitting, setSubmitting] = useState(false);
  const [merged, setMerged] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const close = () => {
    setMerged(false);
    setError(null);
    onClose();
  };

  const onDelete = async () => {
    setSubmitting(true);
    setError(null);
    try {
      await deleteTransaction(txn.id);
      flash("削除しました");
      back("/transactions");
    } catch (e) {
      setSubmitting(false);
      if (e instanceof ApiError && e.code === "ALREADY_MERGED") {
        // マージの統合元は単独では消せない。コアにはマージが無いため、通常は起きない
        setMerged(true);
      } else if (e instanceof ApiError && e.status === 404) {
        // 別のタブで削除済み。削除したときと同じく一覧へ戻る
        flash("すでに削除されています");
        back("/transactions");
      } else if (isRetryable(e)) {
        setError("削除できませんでした。通信状況を確認して、もう一度「削除する」を押してください。");
      } else {
        setError("削除できませんでした。");
      }
    }
  };

  return (
    <Dialog open={open} title="この取引を削除しますか" onClose={close} dismissible={!submitting}>
      {merged ? (
        <>
          <p className="text-sm">この取引は別の取引とまとめられています。マージを解除してから削除してください。</p>
          <DialogActions>
            <Button variant="secondary" data-autofocus onClick={close}>
              閉じる
            </Button>
          </DialogActions>
        </>
      ) : (
        <>
          <div className="flex items-baseline justify-between gap-3 rounded-md bg-bg px-3 py-2 text-sm">
            <span>
              <span className="amount">{formatShortDateTime(txn.occurred_at)}</span> {txn.merchant?.name ?? "店舗名なし"}
            </span>
            <span className="amount font-bold">{formatYen(txn.amount_minor)}</span>
          </div>
          <p className="text-sm">削除すると、一覧と合計から外れます。</p>
          {error && <ErrorBanner>{error}</ErrorBanner>}
          <DialogActions>
            {/* 初期フォーカスは「キャンセル」（Enter の押し間違いで消さないため） */}
            <Button variant="secondary" data-autofocus onClick={close} disabled={submitting}>
              キャンセル
            </Button>
            <Button variant="danger" onClick={onDelete} disabled={submitting}>
              {submitting ? "削除中…" : "削除する"}
            </Button>
          </DialogActions>
        </>
      )}
    </Dialog>
  );
}
