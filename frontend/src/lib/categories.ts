// カテゴリの親子関係を、画面で使う形に並べ替える。
// API-019 は sort_order・id 順の平らな一覧を返すため、親の下に子を並べ直す。

import type { Category } from "@/lib/api/types";

export type CategoryNode = Category & { depth: number };

/**
 * 親の下に子を字下げして並べる（イベント設計 SCR-010・SCR-020「子は親の下に字下げ」）。
 *
 * 親が一覧に無いカテゴリは最上位として扱う。親子が循環していても（通常はサーバが防ぐ）
 * 無限に辿らないよう、出したカテゴリは記録しておく。
 */
export function toCategoryTree(categories: Category[]): CategoryNode[] {
  const ids = new Set(categories.map((c) => c.id));
  const children = new Map<number | null, Category[]>();
  for (const c of categories) {
    const parent = c.parent_id !== null && ids.has(c.parent_id) ? c.parent_id : null;
    const list = children.get(parent) ?? [];
    list.push(c);
    children.set(parent, list);
  }

  const result: CategoryNode[] = [];
  const visited = new Set<number>();
  const walk = (parent: number | null, depth: number) => {
    for (const c of children.get(parent) ?? []) {
      if (visited.has(c.id)) continue;
      visited.add(c.id);
      result.push({ ...c, depth });
      walk(c.id, depth + 1);
    }
  };
  walk(null, 0);

  // 循環していて最上位から辿れなかったものも、落とさずに最上位として出す
  for (const c of categories) {
    if (!visited.has(c.id)) {
      visited.add(c.id);
      result.push({ ...c, depth: 0 });
      walk(c.id, 1);
    }
  }
  return result;
}

/** プルダウンの選択肢の文言。<option> には字下げの要素を入れられないため、全角空白で字下げする。 */
export function indentedName(node: CategoryNode): string {
  return `${"　".repeat(node.depth)}${node.name}`;
}

/** id の子孫（自分は含まない）。 */
export function descendantIds(categories: Category[], id: number): Set<number> {
  const result = new Set<number>();
  const queue = [id];
  while (queue.length > 0) {
    const current = queue.shift()!;
    for (const c of categories) {
      if (c.parent_id === current && !result.has(c.id) && c.id !== id) {
        result.add(c.id);
        queue.push(c.id);
      }
    }
  }
  return result;
}

/** 直下の子カテゴリ。削除すると最上位に移るもの（API-021 DELETE の parent_id の SET NULL）。 */
export function directChildren(categories: Category[], id: number): Category[] {
  return categories.filter((c) => c.parent_id === id);
}
