import type { Category, PaymentMethod } from "@/lib/api/types";
import { indentedName, toCategoryTree } from "@/lib/categories";

/** カテゴリの選択肢。子カテゴリは親の下に字下げする（イベント設計 SCR-010・SCR-011）。 */
export function CategoryOptions({ categories }: { categories: Category[] }) {
  return toCategoryTree(categories).map((node) => (
    <option key={node.id} value={String(node.id)}>
      {indentedName(node)}
    </option>
  ));
}

/** 利用停止中の決済手段は「（利用停止）」を付けて出す（イベント設計 0 章 #4）。 */
export function paymentMethodLabel(method: PaymentMethod): string {
  return method.is_active ? method.name : `${method.name}（利用停止）`;
}

export function PaymentMethodOptions({ methods }: { methods: PaymentMethod[] }) {
  return methods.map((method) => (
    <option key={method.id} value={String(method.id)}>
      {paymentMethodLabel(method)}
    </option>
  ));
}
