import { describe, expect, it } from "vitest";

import type { Category } from "@/lib/api/types";
import { isSafeReturnPath } from "@/lib/auth/session";
import { descendantIds, directChildren, indentedName, toCategoryTree } from "@/lib/categories";
import {
  formatDateTimeWithWeekday,
  formatPeriod,
  isLocalInput,
  isoToLocalInput,
  monthOf,
} from "@/lib/date";
import { formatYen } from "@/lib/format";

describe("isSafeReturnPath（Step 4 判断 7 のオープンリダイレクト対策）", () => {
  it.each(["/transactions", "/transactions?category_id=3", "/settings/categories"])("%s は戻り先にできる", (path) => {
    expect(isSafeReturnPath(path)).toBe(true);
  });

  it.each(["//evil.example", "https://evil.example", "/\\evil.example", "transactions", ""])(
    "%s は戻り先にしない",
    (path) => {
      expect(isSafeReturnPath(path)).toBe(false);
    },
  );
});

describe("日時（JST）", () => {
  it("曜日つきで表示する", () => {
    expect(formatDateTimeWithWeekday("2026-10-06T12:31:00+09:00")).toBe("2026/10/06（火）12:31");
  });

  it("ほかのタイムゾーンの日時も JST の入力欄の値にする", () => {
    expect(isoToLocalInput("2026-09-30T15:30:00Z")).toBe("2026-10-01T00:30");
  });

  it("登録した取引の月（登録後に開く一覧の期間。0 章 #5）", () => {
    expect(monthOf("2026-02-10T12:00:00+09:00")).toEqual({ from: "2026-02-01", to: "2026-02-28" });
  });

  it("対象期間の表示は、同じ年なら終了日の年を省く", () => {
    expect(formatPeriod("2026-10-01", "2026-10-06")).toBe("2026/10/01〜10/06");
    expect(formatPeriod("2025-12-01", "2026-01-31")).toBe("2025/12/01〜2026/01/31");
    expect(formatPeriod(undefined, undefined)).toBe("全期間");
  });

  it("実在しない日時は入力として受け付けない", () => {
    expect(isLocalInput("2026-02-30T10:00")).toBe(false);
    expect(isLocalInput("2026-02-28T24:00")).toBe(false);
    expect(isLocalInput("2026-02-28T23:59")).toBe(true);
  });
});

describe("formatYen", () => {
  it("桁区切りと ¥ を付ける", () => {
    expect(formatYen(38450)).toBe("¥38,450");
  });
});

describe("カテゴリの親子", () => {
  const cat = (id: number, name: string, parent_id: number | null): Category => ({
    id,
    name,
    parent_id,
    sort_order: id,
    is_system: false,
  });
  // 趣味 ─┬ 本 ─ 漫画
  //       └ ゲーム
  const categories = [cat(1, "食費", null), cat(2, "趣味", null), cat(3, "本", 2), cat(4, "ゲーム", 2), cat(5, "漫画", 3)];

  it("親の下に子を並べる", () => {
    expect(toCategoryTree(categories).map(indentedName)).toEqual([
      "食費",
      "趣味",
      "　本",
      "　　漫画",
      "　ゲーム",
    ]);
  });

  it("子孫（親の選択肢から除くもの）", () => {
    expect([...descendantIds(categories, 2)].sort()).toEqual([3, 4, 5]);
  });

  it("削除すると最上位に移るのは直下の子だけ", () => {
    expect(directChildren(categories, 2).map((c) => c.name)).toEqual(["本", "ゲーム"]);
  });

  it("親が一覧に無いカテゴリも落とさない", () => {
    expect(toCategoryTree([cat(7, "孤児", 99)]).map((c) => c.depth)).toEqual([0]);
  });
});
