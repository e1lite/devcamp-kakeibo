// カテゴリの追加・編集ダイアログ（SCR-020）の入力値。検証と、API に送る値の組み立て。

import type { Category, CreateCategoryBody, UpdateCategoryBody } from "@/lib/api/types";

export const CATEGORY_NAME_MAX_LENGTH = 50;

export type CategoryFormValues = {
  name: string;
  // "" は「なし（最上位）」
  parentId: string;
  sortOrder: string;
};

export type CategoryFormErrors = Partial<Record<"name" | "parentId" | "sortOrder", string>>;

export const EMPTY_CATEGORY_FORM: CategoryFormValues = { name: "", parentId: "", sortOrder: "0" };

export function valuesFromCategory(category: Category): CategoryFormValues {
  return {
    name: category.name,
    parentId: category.parent_id === null ? "" : String(category.parent_id),
    sortOrder: String(category.sort_order),
  };
}

/**
 * 名前が空・51 文字以上なら API を呼ばず欄の下に出す（イベント設計 SCR-020 #4）。
 * 文字数はサーバ（Go の rune 数）と合わせ、サロゲートペアを 1 文字と数える。
 */
export function validateCategoryForm(values: CategoryFormValues): CategoryFormErrors {
  const errors: CategoryFormErrors = {};
  const name = values.name.trim();
  if (!name) errors.name = "名前を入力してください";
  else if ([...name].length > CATEGORY_NAME_MAX_LENGTH) errors.name = `${CATEGORY_NAME_MAX_LENGTH} 文字以内で入力してください`;
  if (!/^-?\d+$/.test(values.sortOrder.trim())) errors.sortOrder = "整数で入力してください";
  return errors;
}

/** API-020 に送る値。親が「なし」なら parent_id を送らない。 */
export function toCreateCategoryBody(values: CategoryFormValues): CreateCategoryBody {
  const body: CreateCategoryBody = { name: values.name.trim(), sort_order: Number(values.sortOrder.trim()) };
  if (values.parentId) body.parent_id = Number(values.parentId);
  return body;
}

/** API-021 PUT に送る値。変えた項目だけを送り、親を外すときは parent_id: null（イベント設計 SCR-020 #5）。 */
export function toUpdateCategoryBody(initial: CategoryFormValues, values: CategoryFormValues): UpdateCategoryBody {
  const body: UpdateCategoryBody = {};
  if (values.name.trim() !== initial.name.trim()) body.name = values.name.trim();
  if (values.parentId !== initial.parentId) body.parent_id = values.parentId ? Number(values.parentId) : null;
  if (Number(values.sortOrder.trim()) !== Number(initial.sortOrder)) body.sort_order = Number(values.sortOrder.trim());
  return body;
}
