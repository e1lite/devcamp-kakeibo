import { useId, type ReactNode } from "react";

// 入力欄の共通の枠。ラベルは欄の上、エラーと補足は欄の下（Step 5 のアクセシビリティ）。
//
// 子の入力欄には、render prop で id と aria-describedby を渡す。
// エラーを aria-describedby で欄と結びつけ、読み上げでもどの欄のエラーか分かるようにする。

export type FieldControlProps = {
  id: string;
  "aria-describedby"?: string;
  "aria-invalid"?: boolean;
};

type FieldProps = {
  label: string;
  required?: boolean;
  error?: string;
  // 未来の日時などの、登録は止めない注意
  warning?: string;
  hint?: ReactNode;
  children: (control: FieldControlProps) => ReactNode;
};

export const inputClass =
  "block min-h-11 w-full rounded-md border border-line bg-surface px-3 text-base text-ink " +
  "placeholder:text-muted disabled:opacity-50 aria-[invalid=true]:border-danger";

export function Field({ label, required = false, error, warning, hint, children }: FieldProps) {
  const id = useId();
  const messageId = `${id}-message`;
  const hintId = `${id}-hint`;
  const message = error ?? warning;

  const describedBy = [message ? messageId : null, hint ? hintId : null].filter(Boolean).join(" ") || undefined;

  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="flex items-center gap-2 text-sm font-bold">
        {label}
        {required && <span className="rounded-sm border border-danger px-1 text-xs font-normal text-danger">必須</span>}
      </label>
      {children({ id, "aria-describedby": describedBy, "aria-invalid": error ? true : undefined })}
      {message && (
        <p id={messageId} className={error ? "text-sm text-danger" : "text-sm text-warn"}>
          {error ? `⚠ ${error}` : message}
        </p>
      )}
      {hint && (
        <div id={hintId} className="text-xs text-muted">
          {hint}
        </div>
      )}
    </div>
  );
}
