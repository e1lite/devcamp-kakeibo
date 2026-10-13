"use client";

import { useEffect, useId, useRef, type ReactNode } from "react";

// 確認・入力用のダイアログ。HTML の <dialog> をモーダルとして開く。
//
// showModal() で開くと、外側は操作できなくなり（inert）、Esc で閉じ、
// 閉じたときは開いたボタンへフォーカスが戻る（Step 5 のアクセシビリティ）。
// 初期フォーカスは、中の要素に data-autofocus を付けて決める（削除の確認では「キャンセル」）。
// React の autoFocus は、まだ開いていない（表示されていない）ダイアログの中では効かないため。

type DialogProps = {
  open: boolean;
  title: string;
  // Esc・「キャンセル」・外側の押下で閉じるとき
  onClose: () => void;
  // 送信中など、閉じさせたくないとき
  dismissible?: boolean;
  children: ReactNode;
};

export function Dialog({ open, title, onClose, dismissible = true, children }: DialogProps) {
  const ref = useRef<HTMLDialogElement>(null);
  const openerRef = useRef<HTMLElement | null>(null);
  const titleId = useId();

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) {
      // 閉じたときに戻すため、開いたボタンを覚えておく
      openerRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      dialog.showModal();
      dialog.querySelector<HTMLElement>("[data-autofocus]")?.focus();
    }
    if (!open && dialog.open) {
      dialog.close();
      // 中身は閉じると同時に消えるため、フォーカスしていた要素も消え、ブラウザはフォーカスを戻さない。
      // 開いたボタンへ自分で戻す（Step 5 のアクセシビリティ「閉じたら開いたボタンへフォーカスを戻す」）
      if (openerRef.current?.isConnected) openerRef.current.focus();
      openerRef.current = null;
    }
  }, [open]);

  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      className="m-auto w-[calc(100%-2rem)] max-w-md rounded-lg border border-line bg-surface p-0 text-ink shadow-xl"
      // Esc で閉じるとき。ブラウザに閉じさせず、開閉は open で管理する
      onCancel={(event) => {
        event.preventDefault();
        if (dismissible) onClose();
      }}
      // ダイアログの外（::backdrop）を押したら閉じる。中身の余白の押下では閉じない
      onClick={(event) => {
        if (dismissible && event.target === ref.current) onClose();
      }}
    >
      {open && (
        <div className="flex flex-col gap-4 p-5">
          <h2 id={titleId} className="text-lg font-bold">
            {title}
          </h2>
          {children}
        </div>
      )}
    </dialog>
  );
}

/** ダイアログのボタンの並び。PC は右寄せで横、スマホは主な操作を上にして縦に並べる。 */
export function DialogActions({ children }: { children: ReactNode }) {
  return <div className="flex flex-col-reverse gap-3 pt-2 sm:flex-row sm:justify-end">{children}</div>;
}
