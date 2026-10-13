import Link from "next/link";
import type { ComponentProps } from "react";

// ボタンの見た目は 4 種類に限る（Step 5 のレイアウトの方針）。
// - primary: 塗りつぶし。1 画面に 1 つ、その画面の主な操作だけ
// - secondary: 枠のみ
// - danger: 赤い枠。取り消せない操作。主ボタンから離して置く
// - text: 文字だけ（「再試行」「条件をクリア」など）
type Variant = "primary" | "secondary" | "danger" | "text";

// Tailwind はソースに書かれたクラスだけを生成するため、クラス名は組み立てずに丸ごと書く
const VARIANT_CLASS: Record<Variant, string> = {
  primary: "bg-primary text-primary-ink hover:opacity-90 border border-transparent",
  secondary: "bg-surface text-ink border border-line hover:bg-bg",
  danger: "bg-surface text-danger border border-danger hover:bg-danger-soft",
  text: "text-ink underline underline-offset-4 hover:text-primary border border-transparent",
};

// 押せる範囲は 44px 四方以上（スマホで押し間違えないため）
const BASE_CLASS =
  "inline-flex min-h-11 items-center justify-center gap-1 rounded-md px-4 text-sm font-bold whitespace-nowrap " +
  "disabled:cursor-not-allowed disabled:opacity-45";

export function buttonClass(variant: Variant = "secondary", extra = ""): string {
  return `${BASE_CLASS} ${VARIANT_CLASS[variant]} ${extra}`.trim();
}

type ButtonProps = ComponentProps<"button"> & { variant?: Variant };

export function Button({ variant = "secondary", className = "", type = "button", ...props }: ButtonProps) {
  return <button type={type} className={buttonClass(variant, className)} {...props} />;
}

type LinkButtonProps = ComponentProps<typeof Link> & { variant?: Variant };

export function LinkButton({ variant = "secondary", className = "", ...props }: LinkButtonProps) {
  return <Link className={buttonClass(variant, className)} {...props} />;
}
