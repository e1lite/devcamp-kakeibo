"use client";

import { useEffect, useRef, useState } from "react";

import { Page, useApp } from "@/components/app/AppShell";
import { Button } from "@/components/Button";
import { Dialog, DialogActions } from "@/components/Dialog";
import { EmptyState, ErrorBanner, LoadError, Skeleton } from "@/components/Feedback";
import { ApiError, isRetryable } from "@/lib/api/client";
import { disconnectMailAccount, listMailAccounts, listSyncJobs, requestSync, startMailLink } from "@/lib/api/endpoints";
import type { MailAccount, MailAccountStatus, SyncJob, SyncJobStatus } from "@/lib/api/types";
import { formatDateTime } from "@/lib/date";
import { useResource } from "@/lib/useResource";

// SCR-050 メール連携（最小版。docs/mail-import-mvp.md 6 章）。
// 決済メールを読む Gmail を連携し、手動で同期する。

type AccountRow = MailAccount & { latestJob: SyncJob | null };

// 連携の一覧と、それぞれの最新の同期結果（API-009 を limit=1 で）を取る
async function loadAccounts(): Promise<AccountRow[]> {
  const accounts = await listMailAccounts();
  const jobs = await Promise.all(accounts.map((account) => listSyncJobs(account.id, 1)));
  return accounts.map((account, i) => ({ ...account, latestJob: jobs[i][0] ?? null }));
}

const STATUS_LABEL: Record<MailAccountStatus, string> = {
  active: "連携中",
  reauth_required: "再認証が必要",
  disabled: "解除済み",
};

const JOB_LABEL: Record<SyncJobStatus, string> = {
  running: "取り込み中…",
  success: "成功",
  partial: "一部を取り込めませんでした",
  failed: "失敗",
};

/**
 * API-030 から戻ったときの結果（フラグメントの code）→ 表示する文言。
 * 文言はこの表から選び、URL の文字列をそのまま画面に出さない（ログイン画面と同じ方針）。
 */
const LINK_ERRORS: Record<string, string> = {
  ACCESS_DENIED: "Gmail の読み取りが許可されませんでした。",
  INSUFFICIENT_SCOPE: "同意画面で「Gmail のメールの表示」にチェックを入れて、もう一度連携してください。",
  INVALID_STATE: "連携をやり直してください。",
  VALIDATION_ERROR: "連携をやり直してください。",
  OAUTH_EXCHANGE_FAILED: "Google との通信に失敗しました。時間をおいてもう一度お試しください。",
};

// 同期の進み具合を確かめる間隔
const POLL_MS = 3000;

export function MailAccountsView() {
  const { flash } = useApp();
  const accounts = useResource(loadAccounts);
  const [message, setMessage] = useState<string | null>(null);
  const [connecting, setConnecting] = useState(false);
  const [syncingId, setSyncingId] = useState<number | null>(null);
  const [disconnecting, setDisconnecting] = useState<AccountRow | null>(null);
  const handledFragment = useRef(false);

  // API-030 から戻ったとき、フラグメントの結果を読んで URL から消す
  useEffect(() => {
    if (handledFragment.current) return;
    handledFragment.current = true;

    const fragment = new URLSearchParams(window.location.hash.slice(1));
    const result = fragment.get("mail_link");
    if (!result) return;
    window.history.replaceState(window.history.state, "", window.location.pathname);

    if (result === "success") {
      flash("Gmail を連携しました。過去のメールを取り込んでいます", "here");
    } else {
      // 外部（URL）の値を読んだ結果を、一度だけ画面に反映する
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setMessage(LINK_ERRORS[fragment.get("code") ?? ""] ?? "連携できませんでした。もう一度お試しください。");
    }
  }, [flash]);

  // 取り込み中の同期があれば、終わるまで数秒ごとに取り直す
  const { refresh } = accounts;
  const anyRunning =
    accounts.state.status === "success" && accounts.state.data.some((a) => a.latestJob?.status === "running");
  useEffect(() => {
    if (!anyRunning) return;
    const timer = window.setTimeout(refresh, POLL_MS);
    return () => window.clearTimeout(timer);
  }, [anyRunning, accounts.state, refresh]);

  // Gmail を連携・再認証する。API-029 で受け取った URL へブラウザごと移る
  const onConnect = async () => {
    setConnecting(true);
    setMessage(null);
    try {
      const { authorization_url } = await startMailLink(`${window.location.origin}/settings/mail-accounts`);
      window.location.assign(authorization_url);
    } catch {
      setConnecting(false);
      setMessage("連携を始められませんでした。時間をおいてもう一度お試しください。");
    }
  };

  const onSync = async (account: AccountRow) => {
    setSyncingId(account.id);
    setMessage(null);
    try {
      await requestSync(account.id);
    } catch (error) {
      if (error instanceof ApiError && error.code === "REAUTH_REQUIRED") {
        setMessage("連携の有効期限が切れています。「再認証」を押して、もう一度連携してください。");
      } else if (error instanceof ApiError && error.code === "SYNC_ALREADY_RUNNING") {
        // すでに取り込み中。一覧を取り直せば進み具合が見える
      } else if (isRetryable(error)) {
        setMessage("同期を始められませんでした。通信状況を確認して、もう一度お試しください。");
      } else {
        setMessage("同期を始められませんでした。");
      }
    } finally {
      setSyncingId(null);
      refresh();
    }
  };

  return (
    <Page width="list">
      <div className="flex flex-col gap-2">
        <div className="flex items-center justify-between gap-4">
          <h1 className="text-2xl font-bold">メール連携</h1>
          <Button variant="primary" onClick={onConnect} disabled={connecting}>
            {connecting ? "Google に移動しています…" : "Gmail を連携"}
          </Button>
        </div>
        <p className="text-sm leading-relaxed text-muted">
          カード会社の利用通知メールを Gmail から取り込みます。連携で許可するのはメールの読み取りだけです。
          取り込むのは、件名が「〜利用のお知らせ」などの通知メールだけです。
        </p>
      </div>

      {message && <ErrorBanner>{message}</ErrorBanner>}

      {accounts.state.status === "loading" && (
        <div className="rounded-md border border-line bg-surface p-4">
          <Skeleton rows={3} />
        </div>
      )}

      {accounts.state.status === "error" && (
        <LoadError message="連携を読み込めませんでした。" onRetry={accounts.retry} />
      )}

      {accounts.state.status === "success" &&
        (accounts.state.data.length === 0 ? (
          <EmptyState
            title="連携しているメールはありません"
            description="「Gmail を連携」から、カードの利用通知が届く Gmail を連携してください。"
          />
        ) : (
          <ul className="flex flex-col gap-3">
            {accounts.state.data.map((account) => (
              <AccountItem
                key={account.id}
                account={account}
                syncing={syncingId === account.id}
                onSync={() => void onSync(account)}
                onReconnect={() => void onConnect()}
                onDisconnect={() => setDisconnecting(account)}
              />
            ))}
          </ul>
        ))}

      <Dialog
        open={disconnecting !== null}
        title="連携を解除しますか"
        onClose={() => setDisconnecting(null)}
      >
        {disconnecting && (
          <DisconnectConfirm
            account={disconnecting}
            onCancel={() => setDisconnecting(null)}
            onDone={() => {
              setDisconnecting(null);
              refresh();
              flash("連携を解除しました", "here");
            }}
          />
        )}
      </Dialog>
    </Page>
  );
}

type AccountItemProps = {
  account: AccountRow;
  syncing: boolean;
  onSync: () => void;
  onReconnect: () => void;
  onDisconnect: () => void;
};

function AccountItem({ account, syncing, onSync, onReconnect, onDisconnect }: AccountItemProps) {
  const job = account.latestJob;
  const running = job?.status === "running";

  return (
    <li className="flex flex-col gap-3 rounded-md border border-line bg-surface p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="font-bold break-all">{account.email_address}</span>
        {/* 状態は色だけでなく文字でも示す */}
        <span
          className={
            account.status === "reauth_required"
              ? "rounded-sm border border-warn px-2 text-xs text-warn"
              : "rounded-sm border border-line px-2 text-xs text-muted"
          }
        >
          {STATUS_LABEL[account.status]}
        </span>
      </div>

      <dl className="grid grid-cols-[7rem_1fr] gap-x-4 gap-y-1 text-sm">
        <dt className="text-muted">最終同期</dt>
        <dd className="amount">{account.last_synced_at ? formatDateTime(account.last_synced_at) : "まだ同期していません"}</dd>
        <dt className="text-muted">最新の同期</dt>
        <dd>
          {job ? (
            <>
              <span className="amount">{formatDateTime(job.started_at)}</span> {JOB_LABEL[job.status]}
              {job.status !== "running" && (
                <span className="text-muted">
                  （取得 {job.fetched_count} 件{job.failed_count > 0 && `・失敗 ${job.failed_count} 件`}）
                </span>
              )}
              {job.error_message && <span className="block text-xs text-danger">{job.error_message}</span>}
            </>
          ) : (
            "なし"
          )}
        </dd>
      </dl>

      <div className="flex flex-wrap gap-2">
        {account.status === "active" && (
          <Button variant="secondary" onClick={onSync} disabled={syncing || running}>
            {running ? "取り込み中…" : syncing ? "送信中…" : "今すぐ同期"}
          </Button>
        )}
        {account.status !== "active" && (
          <Button variant="secondary" onClick={onReconnect}>
            {account.status === "reauth_required" ? "再認証" : "もう一度連携"}
          </Button>
        )}
        {account.status !== "disabled" && (
          <Button variant="danger" onClick={onDisconnect} className="ml-auto">
            連携を解除
          </Button>
        )}
      </div>
    </li>
  );
}

/** 連携解除の確認（API-007）。取り込んだメールと家計簿は残ることを伝える。 */
function DisconnectConfirm({ account, onCancel, onDone }: { account: AccountRow; onCancel: () => void; onDone: () => void }) {
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const onConfirm = async () => {
    setSubmitting(true);
    setError(null);
    try {
      await disconnectMailAccount(account.id);
      onDone();
    } catch (e) {
      setSubmitting(false);
      if (e instanceof ApiError && (e.code === "ALREADY_DISABLED" || e.code === "MAIL_ACCOUNT_NOT_FOUND")) onDone();
      else setError("解除できませんでした。もう一度お試しください。");
    }
  };

  return (
    <>
      <p className="text-sm leading-relaxed">
        <span className="font-bold break-all">{account.email_address}</span> のメールを読まなくなります。
        これまでに取り込んだメールと取引は残ります。
      </p>
      {error && <ErrorBanner>{error}</ErrorBanner>}
      <DialogActions>
        <Button variant="secondary" data-autofocus onClick={onCancel} disabled={submitting}>
          キャンセル
        </Button>
        <Button variant="danger" onClick={onConfirm} disabled={submitting}>
          {submitting ? "解除中…" : "解除する"}
        </Button>
      </DialogActions>
    </>
  );
}
