import Link from "next/link";

import type { TransactionSummary } from "@/lib/api/types";
import { formatShortDateTime } from "@/lib/date";
import { formatYen } from "@/lib/format";

// 取引一覧の D 一覧。行全体を取引詳細へのリンクにする（Tab で移動でき、Enter で開ける）。
// PC は 5 列の表、スマホはカード（1 行目に店舗名と金額、2 行目に日時・カテゴリ・決済手段）。
// 5 列の表は 360px の幅に収まらず、店舗名と金額が読めなくなるため（Step 5 の判断 3）。

const COLUMNS = "md:grid-cols-[6.5rem_minmax(0,1fr)_8rem_8rem_8rem]";

/** 速報の印。確定前の金額であることを、色と文字の両方で示す（色だけで意味を伝えない）。 */
function PendingBadge() {
  return (
    <span className="mr-2 rounded-sm border border-warn px-1 text-xs font-normal text-warn" title="確定前の金額です">
      速報
    </span>
  );
}

function MerchantName({ txn }: { txn: TransactionSummary }) {
  return txn.merchant ? <>{txn.merchant.name}</> : <span className="text-muted">店舗名なし</span>;
}

function CategoryName({ txn }: { txn: TransactionSummary }) {
  // category が null の取引は「未分類」（API 仕様書 API-003 の注記）
  return txn.category ? <>{txn.category.name}</> : <span className="text-muted">未分類</span>;
}

function PaymentMethodName({ txn }: { txn: TransactionSummary }) {
  return txn.payment_method ? <>{txn.payment_method.name}</> : <span className="text-muted">なし</span>;
}

export function TransactionRows({ rows }: { rows: TransactionSummary[] }) {
  return (
    <div className="overflow-hidden rounded-md border border-line bg-surface">
      {/* PC の表の見出し。スマホのカードでは出さない */}
      <div
        aria-hidden="true"
        className={`hidden border-b border-line px-4 py-2 text-xs font-bold text-muted md:grid ${COLUMNS} md:gap-3`}
      >
        <span>日時</span>
        <span>店舗</span>
        <span>カテゴリ</span>
        <span>決済手段</span>
        <span className="text-right">金額</span>
      </div>

      <ul>
        {rows.map((txn) => (
          <li key={txn.id} className="border-b border-line last:border-b-0">
            <Link href={`/transactions/${txn.id}`} className="block px-4 py-3 hover:bg-bg">
              {/* スマホ: カード */}
              <div className="flex flex-col gap-1 md:hidden">
                <div className="flex items-baseline justify-between gap-3">
                  <span className="truncate font-bold">
                    <MerchantName txn={txn} />
                  </span>
                  <span className="amount shrink-0 font-bold">
                    {txn.status === "pending" && <PendingBadge />}
                    {formatYen(txn.amount_minor)}
                  </span>
                </div>
                <div className="text-xs text-muted">
                  {formatShortDateTime(txn.occurred_at)} · <CategoryName txn={txn} /> · <PaymentMethodName txn={txn} />
                </div>
              </div>

              {/* PC: 表の行 */}
              <div className={`hidden items-center gap-3 text-sm md:grid ${COLUMNS}`}>
                <span className="amount">{formatShortDateTime(txn.occurred_at)}</span>
                <span className="truncate">
                  <MerchantName txn={txn} />
                </span>
                <span className="truncate">
                  <CategoryName txn={txn} />
                </span>
                <span className="truncate">
                  <PaymentMethodName txn={txn} />
                </span>
                {/* 金額は右揃え・桁区切り・等幅数字 */}
                <span className="amount text-right font-bold">
                  {txn.status === "pending" && <PendingBadge />}
                  {formatYen(txn.amount_minor)}
                </span>
              </div>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
