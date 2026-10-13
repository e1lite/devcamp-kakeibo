const yen = new Intl.NumberFormat("ja-JP");

/** 金額を ¥38,450 の形式にする（イベント設計 1.2）。amount_minor は円の整数。 */
export function formatYen(amountMinor: number): string {
  return `¥${yen.format(amountMinor)}`;
}
