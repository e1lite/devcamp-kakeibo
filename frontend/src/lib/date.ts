// 日時は日本時間（JST, UTC+9）で扱う。「何月の支出か」は JST で決める（DB 仕様書 1 章）。
//
// ブラウザのタイムゾーンに左右されないよう、Date の UTC の値を 9 時間ずらして読む。
// JST には夏時間が無いため、固定のずれで正しく変換できる。

const JST_OFFSET_MS = 9 * 60 * 60 * 1000;
const WEEKDAYS = ["日", "月", "火", "水", "木", "金", "土"];

type JstParts = {
  year: number;
  month: number;
  day: number;
  hour: number;
  minute: number;
  weekday: number;
};

export function jstParts(date: Date): JstParts {
  const d = new Date(date.getTime() + JST_OFFSET_MS);
  return {
    year: d.getUTCFullYear(),
    month: d.getUTCMonth() + 1,
    day: d.getUTCDate(),
    hour: d.getUTCHours(),
    minute: d.getUTCMinutes(),
    weekday: d.getUTCDay(),
  };
}

const pad2 = (n: number) => String(n).padStart(2, "0");

function ymd(year: number, month: number, day: number): string {
  return `${year}-${pad2(month)}-${pad2(day)}`;
}

export function lastDayOfMonth(year: number, month: number): number {
  // 翌月の 0 日 = その月の末日
  return new Date(Date.UTC(year, month, 0)).getUTCDate();
}

/** 今日の日付（JST、YYYY-MM-DD）。 */
export function todayJst(now: Date = new Date()): string {
  const p = jstParts(now);
  return ymd(p.year, p.month, p.day);
}

/** 取引一覧の既定の期間：当月 1 日〜今日（イベント設計 SCR-010）。 */
export function currentMonthToDate(now: Date = new Date()): { from: string; to: string } {
  const p = jstParts(now);
  return { from: ymd(p.year, p.month, 1), to: ymd(p.year, p.month, p.day) };
}

/** 日時（ISO 8601）が属する月の 1 日と末日（JST）。登録後の一覧の期間に使う（イベント設計 0 章 #5）。 */
export function monthOf(iso: string): { from: string; to: string } {
  const p = jstParts(new Date(iso));
  return { from: ymd(p.year, p.month, 1), to: ymd(p.year, p.month, lastDayOfMonth(p.year, p.month)) };
}

/** YYYY-MM-DD として実在する日付か。 */
export function isDateString(value: string): boolean {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!m) return false;
  const [, y, mo, d] = m.map(Number);
  return mo >= 1 && mo <= 12 && d >= 1 && d <= lastDayOfMonth(y, mo);
}

// --- 入力欄（datetime-local）との変換 ------------------------------------

/** ISO 8601 の日時を、datetime-local の入力欄の値（JST の YYYY-MM-DDTHH:mm）にする。 */
export function isoToLocalInput(iso: string): string {
  const p = jstParts(new Date(iso));
  return `${ymd(p.year, p.month, p.day)}T${pad2(p.hour)}:${pad2(p.minute)}`;
}

/** 入力欄の値を、API に送る ISO 8601（+09:00 付き）にする（イベント設計 SCR-011 の送り方）。 */
export function localInputToIso(local: string): string {
  return `${local}:00+09:00`;
}

/** 入力欄の値として正しい形か（YYYY-MM-DDTHH:mm で実在する日時）。 */
export function isLocalInput(value: string): boolean {
  const m = /^(\d{4}-\d{2}-\d{2})T(\d{2}):(\d{2})$/.exec(value);
  return m !== null && isDateString(m[1]) && Number(m[2]) < 24 && Number(m[3]) < 60;
}

/** 今の日時を入力欄の値にする（取引登録の初期値）。 */
export function nowLocalInput(now: Date = new Date()): string {
  return isoToLocalInput(now.toISOString());
}

/** 入力欄の日時が現在より後か。未来の日時は警告だけ出す（イベント設計 SCR-011 #2）。 */
export function isFutureLocalInput(local: string, now: Date = new Date()): boolean {
  return new Date(localInputToIso(local)).getTime() > now.getTime();
}

// --- 表示 ------------------------------------------------------------------

/** 2026/09/09 12:31（イベント設計 1.2 の表示形式）。 */
export function formatDateTime(iso: string): string {
  const p = jstParts(new Date(iso));
  return `${p.year}/${pad2(p.month)}/${pad2(p.day)} ${pad2(p.hour)}:${pad2(p.minute)}`;
}

/** 09/09 12:31（一覧の行）。 */
export function formatShortDateTime(iso: string): string {
  const p = jstParts(new Date(iso));
  return `${pad2(p.month)}/${pad2(p.day)} ${pad2(p.hour)}:${pad2(p.minute)}`;
}

/** 2026/10/06（火）12:31（取引詳細。曜日を添えて思い出しやすくする）。 */
export function formatDateTimeWithWeekday(iso: string): string {
  const p = jstParts(new Date(iso));
  return `${p.year}/${pad2(p.month)}/${pad2(p.day)}（${WEEKDAYS[p.weekday]}）${pad2(p.hour)}:${pad2(p.minute)}`;
}

/** YYYY-MM-DD → 2026/10/01。 */
export function formatDate(value: string): string {
  return value.replaceAll("-", "/");
}

/**
 * 集計に添える対象期間。2026/10/01〜10/06 のように、同じ年なら終了日の年を省く。
 * 期間が無ければ「全期間」（店舗で絞り込んだとき。イベント設計 0 章 #2）。
 */
export function formatPeriod(from?: string, to?: string): string {
  if (!from && !to) return "全期間";
  if (from && !to) return `${formatDate(from)}〜`;
  if (!from && to) return `〜${formatDate(to)}`;
  const f = formatDate(from!);
  const t = formatDate(to!);
  return from!.slice(0, 4) === to!.slice(0, 4) ? `${f}〜${t.slice(5)}` : `${f}〜${t}`;
}
