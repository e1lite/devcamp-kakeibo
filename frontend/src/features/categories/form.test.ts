import { describe, expect, it } from "vitest";

import {
  EMPTY_CATEGORY_FORM,
  toCreateCategoryBody,
  toUpdateCategoryBody,
  validateCategoryForm,
  valuesFromCategory,
} from "@/features/categories/form";

describe("validateCategoryForm", () => {
  it("名前が空なら API を呼ばずに知らせる", () => {
    expect(validateCategoryForm({ ...EMPTY_CATEGORY_FORM, name: "  " }).name).toBe("名前を入力してください");
  });

  it("名前は 50 文字まで（51 文字以上はエラー）", () => {
    expect(validateCategoryForm({ ...EMPTY_CATEGORY_FORM, name: "あ".repeat(50) })).toEqual({});
    expect(validateCategoryForm({ ...EMPTY_CATEGORY_FORM, name: "あ".repeat(51) }).name).toBe(
      "50 文字以内で入力してください",
    );
  });

  it("表示順は整数", () => {
    expect(validateCategoryForm({ name: "交際費", parentId: "", sortOrder: "1.5" }).sortOrder).toBe(
      "整数で入力してください",
    );
    expect(validateCategoryForm({ name: "交際費", parentId: "", sortOrder: "-1" })).toEqual({});
  });
});

describe("toCreateCategoryBody", () => {
  it("親が「なし」なら parent_id を送らない", () => {
    expect(toCreateCategoryBody({ name: " 交際費 ", parentId: "", sortOrder: "10" })).toEqual({
      name: "交際費",
      sort_order: 10,
    });
  });
});

describe("toUpdateCategoryBody（変えた項目だけを送る）", () => {
  const initial = valuesFromCategory({ id: 3, name: "本", parent_id: 2, sort_order: 1, is_system: false });

  it("何も変えていなければ空", () => {
    expect(toUpdateCategoryBody(initial, { ...initial })).toEqual({});
  });

  it("親を外すときは parent_id: null", () => {
    expect(toUpdateCategoryBody(initial, { ...initial, parentId: "" })).toEqual({ parent_id: null });
  });

  it("名前と表示順を変えた", () => {
    expect(toUpdateCategoryBody(initial, { ...initial, name: "書籍", sortOrder: "5" })).toEqual({
      name: "書籍",
      sort_order: 5,
    });
  });
});
