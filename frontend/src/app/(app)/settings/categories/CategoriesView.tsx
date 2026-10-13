"use client";

import { useState, type FormEvent } from "react";

import { Page, useApp } from "@/components/app/AppShell";
import { Button } from "@/components/Button";
import { Dialog, DialogActions } from "@/components/Dialog";
import { EmptyState, ErrorBanner, LoadError, operationErrorMessage, Skeleton } from "@/components/Feedback";
import { Field, inputClass } from "@/components/Field";
import {
  EMPTY_CATEGORY_FORM,
  toCreateCategoryBody,
  toUpdateCategoryBody,
  validateCategoryForm,
  valuesFromCategory,
  type CategoryFormErrors,
  type CategoryFormValues,
} from "@/features/categories/form";
import { ApiError, isRetryable } from "@/lib/api/client";
import { createCategory, deleteCategory, listCategories, updateCategory } from "@/lib/api/endpoints";
import type { Category } from "@/lib/api/types";
import { descendantIds, directChildren, indentedName, toCategoryTree } from "@/lib/categories";
import { useResource } from "@/lib/useResource";

const loadCategories = () => listCategories({ includeCounts: true });

type DialogState =
  | { kind: "create" }
  | { kind: "edit"; category: Category }
  | { kind: "delete"; category: Category }
  | null;

/**
 * SCR-020 カテゴリ管理。削除の影響（取引が未分類になる）を、削除する前に分かるようにする。
 *
 * 作成・更新・削除のあとは、画面上で一覧を書き換えるのではなく API-019 を取り直す。
 * 並び順・親子関係・取引件数をサーバの結果に合わせるため（イベント設計 SCR-020）。
 */
export function CategoriesView() {
  const { flash } = useApp();
  const categories = useResource(loadCategories);
  const [dialog, setDialog] = useState<DialogState>(null);
  const [pageError, setPageError] = useState<string | null>(null);

  // 成功したら、ダイアログを閉じて一覧を取り直し、完了メッセージをこの画面に出す
  const done = (message: string) => {
    setDialog(null);
    setPageError(null);
    categories.refresh();
    flash(message, "here");
  };

  // 404 CATEGORY_NOT_FOUND。別のタブで削除されていた
  const gone = () => {
    setDialog(null);
    setPageError("このカテゴリはすでに削除されています。");
    categories.refresh();
  };

  const list = categories.state.status === "success" ? categories.state.data : [];

  return (
    <Page width="list">
      <div className="flex flex-col gap-2">
        <div className="flex items-center justify-between gap-4">
          <h1 className="text-2xl font-bold">カテゴリ</h1>
          <Button variant="primary" onClick={() => setDialog({ kind: "create" })}>
            ＋ 追加
          </Button>
        </div>
        <p className="text-sm text-muted">
          取引の分類に使います。カテゴリを削除すると、そのカテゴリの取引は「未分類」になります。
        </p>
      </div>

      {pageError && <ErrorBanner>{pageError}</ErrorBanner>}

      {categories.state.status === "loading" && (
        <div className="rounded-md border border-line bg-surface p-4">
          <Skeleton rows={6} />
        </div>
      )}

      {categories.state.status === "error" && (
        <LoadError message="カテゴリを読み込めませんでした。" onRetry={categories.retry} />
      )}

      {categories.state.status === "success" &&
        (list.length === 0 ? (
          <EmptyState
            title="カテゴリがありません"
            description="「追加」から、取引の分類に使うカテゴリを作れます。"
          />
        ) : (
          <CategoryTable categories={list} onEdit={(c) => setDialog({ kind: "edit", category: c })} onDelete={(c) => setDialog({ kind: "delete", category: c })} />
        ))}

      <Dialog
        open={dialog?.kind === "create" || dialog?.kind === "edit"}
        title={dialog?.kind === "edit" ? "カテゴリの編集" : "カテゴリの追加"}
        onClose={() => setDialog(null)}
      >
        {(dialog?.kind === "create" || dialog?.kind === "edit") && (
          <CategoryForm
            category={dialog.kind === "edit" ? dialog.category : null}
            categories={list}
            onCancel={() => setDialog(null)}
            onDone={done}
            onGone={gone}
            onParentMissing={categories.refresh}
          />
        )}
      </Dialog>

      <Dialog
        open={dialog?.kind === "delete"}
        title={dialog?.kind === "delete" ? `「${dialog.category.name}」を削除しますか` : ""}
        onClose={() => setDialog(null)}
      >
        {dialog?.kind === "delete" && (
          <DeleteCategory
            category={dialog.category}
            categories={list}
            onCancel={() => setDialog(null)}
            onDone={done}
            onGone={gone}
          />
        )}
      </Dialog>
    </Page>
  );
}

/**
 * B 一覧。カテゴリ名・取引件数・操作の 3 列。表示順のとおりに並べ、子は親の下に字下げする。
 * 初期カテゴリもほかのカテゴリと同じく編集・削除でき、印も付けない（Step 4 のフィードバック）。
 * 「未分類」は一覧に行として出さない（カテゴリとして存在しないため）。
 */
function CategoryTable({
  categories,
  onEdit,
  onDelete,
}: {
  categories: Category[];
  onEdit: (category: Category) => void;
  onDelete: (category: Category) => void;
}) {
  return (
    <table className="w-full border-collapse overflow-hidden rounded-md border border-line bg-surface text-sm">
      <thead>
        <tr className="border-b border-line text-left text-xs text-muted">
          <th scope="col" className="px-4 py-2 font-bold">
            カテゴリ
          </th>
          <th scope="col" className="hidden px-4 py-2 text-right font-bold md:table-cell">
            取引
          </th>
          <th scope="col" className="px-4 py-2 text-right font-bold">
            操作
          </th>
        </tr>
      </thead>
      <tbody>
        {toCategoryTree(categories).map((node) => (
          <tr key={node.id} className="border-b border-line last:border-b-0">
            <td className="px-4 py-2">
              {/* 親子関係を字下げで示す */}
              <div style={{ paddingLeft: `${node.depth * 1.5}rem` }}>
                <span className="font-bold">{node.name}</span>
                {/* スマホでは件数をカテゴリ名の下に移す */}
                <span className="amount block text-xs text-muted md:hidden">{node.transaction_count ?? 0} 件</span>
              </div>
            </td>
            {/* 削除したときに未分類になる件数の目安。右揃え */}
            <td className="amount hidden px-4 py-2 text-right md:table-cell">{node.transaction_count ?? 0} 件</td>
            <td className="px-2 py-1 text-right whitespace-nowrap">
              <button
                type="button"
                onClick={() => onEdit(node)}
                className="inline-flex min-h-11 min-w-11 items-center justify-center px-2 underline underline-offset-4"
                aria-label={`「${node.name}」を編集`}
              >
                編集
              </button>
              <button
                type="button"
                onClick={() => onDelete(node)}
                className="inline-flex min-h-11 min-w-11 items-center justify-center px-2 text-danger underline underline-offset-4"
                aria-label={`「${node.name}」を削除`}
              >
                削除
              </button>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

type CategoryFormProps = {
  // null なら追加
  category: Category | null;
  categories: Category[];
  onCancel: () => void;
  onDone: (message: string) => void;
  onGone: () => void;
  onParentMissing: () => void;
};

/** 追加・編集ダイアログの中身（SCR-020 #2〜#5）。ダイアログを開くたびに作り直され、入力は残らない。 */
function CategoryForm({ category, categories, onCancel, onDone, onGone, onParentMissing }: CategoryFormProps) {
  const initial = category ? valuesFromCategory(category) : EMPTY_CATEGORY_FORM;
  const [values, setValues] = useState<CategoryFormValues>(initial);
  const [errors, setErrors] = useState<CategoryFormErrors>({});
  const [message, setMessage] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // 親の選択肢から、自分自身と子孫を除く（サーバの 400 INVALID_PARENT に頼る前に防ぐ。SCR-020 #3）
  const excluded = category ? new Set([category.id, ...descendantIds(categories, category.id)]) : new Set<number>();
  const parentOptions = toCategoryTree(categories).filter((node) => !excluded.has(node.id));

  const set = (key: keyof CategoryFormValues) => (value: string) => setValues((v) => ({ ...v, [key]: value }));

  const onSubmit = async (event: FormEvent) => {
    event.preventDefault();
    const found = validateCategoryForm(values);
    setErrors(found);
    if (Object.keys(found).length > 0) return;

    setSubmitting(true);
    setMessage(null);
    try {
      if (category) {
        const body = toUpdateCategoryBody(initial, values);
        // 何も変えていなければ API を呼ばずに閉じる
        if (Object.keys(body).length === 0) {
          onCancel();
          return;
        }
        await updateCategory(category.id, body);
        onDone("保存しました");
      } else {
        await createCategory(toCreateCategoryBody(values));
        onDone("追加しました");
      }
    } catch (error) {
      setSubmitting(false);
      if (isRetryable(error) || !(error instanceof ApiError)) {
        setMessage(operationErrorMessage("保存"));
        return;
      }
      switch (error.code) {
        case "DUPLICATE_CATEGORY_NAME":
          setErrors({ name: "同じ名前のカテゴリがあります" });
          break;
        case "VALIDATION_ERROR":
          setErrors({ name: "名前を確認してください" });
          break;
        case "INVALID_PARENT":
          setErrors({ parentId: "自分自身や、その下のカテゴリは親にできません" });
          break;
        case "PARENT_NOT_FOUND":
          // 親が別のタブで削除されていた。一覧（選択肢）を取り直す
          setErrors({ parentId: "親カテゴリが見つかりません" });
          setValues((v) => ({ ...v, parentId: "" }));
          onParentMissing();
          break;
        case "CATEGORY_NOT_FOUND":
          onGone();
          break;
        default:
          setMessage("保存できませんでした。入力内容を確認してください。");
      }
    }
  };

  return (
    <form onSubmit={onSubmit} noValidate className="flex flex-col gap-4">
      {message && <ErrorBanner>{message}</ErrorBanner>}

      <Field label="名前" required error={errors.name}>
        {(control) => (
          <input
            {...control}
            data-autofocus
            type="text"
            placeholder="例: 交際費"
            className={inputClass}
            value={values.name}
            onChange={(e) => set("name")(e.target.value)}
          />
        )}
      </Field>

      <Field label="親カテゴリ" error={errors.parentId}>
        {(control) => (
          <select {...control} className={inputClass} value={values.parentId} onChange={(e) => set("parentId")(e.target.value)}>
            <option value="">なし（最上位）</option>
            {parentOptions.map((node) => (
              <option key={node.id} value={String(node.id)}>
                {indentedName(node)}
              </option>
            ))}
          </select>
        )}
      </Field>

      <Field label="表示順" error={errors.sortOrder} hint="小さい数ほど上に表示します">
        {(control) => (
          <input
            {...control}
            type="text"
            inputMode="numeric"
            className={`${inputClass} amount`}
            value={values.sortOrder}
            onChange={(e) => set("sortOrder")(e.target.value)}
          />
        )}
      </Field>

      <DialogActions>
        <Button variant="secondary" onClick={onCancel} disabled={submitting}>
          キャンセル
        </Button>
        <Button type="submit" variant="primary" disabled={submitting}>
          {submitting ? "送信中…" : "保存"}
        </Button>
      </DialogActions>
    </form>
  );
}

type DeleteCategoryProps = {
  category: Category;
  categories: Category[];
  onCancel: () => void;
  onDone: (message: string) => void;
  onGone: () => void;
};

/**
 * 削除の確認（SCR-020 #6・#7）。未分類になる取引の件数と、最上位に移る子カテゴリを示す。
 * 取引件数だけを示すと、階層が崩れることに気づけないため（イベント設計 0 章 #7）。
 */
function DeleteCategory({ category, categories, onCancel, onDone, onGone }: DeleteCategoryProps) {
  const [submitting, setSubmitting] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const children = directChildren(categories, category.id);
  const count = category.transaction_count ?? 0;

  const onDelete = async () => {
    setSubmitting(true);
    setMessage(null);
    try {
      await deleteCategory(category.id);
      onDone("削除しました");
    } catch (error) {
      setSubmitting(false);
      if (error instanceof ApiError && error.code === "CATEGORY_NOT_FOUND") onGone();
      else if (isRetryable(error)) {
        setMessage("削除できませんでした。通信状況を確認して、もう一度「削除する」を押してください。");
      } else setMessage("削除できませんでした。");
    }
  };

  return (
    <>
      <p className="text-sm leading-relaxed">
        このカテゴリの取引 <span className="amount font-bold">{count}</span> 件は「未分類」になります。
        {children.length > 0 && (
          <>
            子カテゴリ {children.length} 件（{children.map((c) => c.name).join("・")}）は最上位に移ります。
          </>
        )}
      </p>
      {message && <ErrorBanner>{message}</ErrorBanner>}
      <DialogActions>
        {/* 初期フォーカスは「キャンセル」 */}
        <Button variant="secondary" data-autofocus onClick={onCancel} disabled={submitting}>
          キャンセル
        </Button>
        <Button variant="danger" onClick={onDelete} disabled={submitting}>
          {submitting ? "削除中…" : "削除する"}
        </Button>
      </DialogActions>
    </>
  );
}
