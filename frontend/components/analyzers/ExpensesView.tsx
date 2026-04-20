import type { ExpensesStructured } from "@/lib/types";

interface Props {
  data: ExpensesStructured;
}

export function ExpensesView({ data }: Props) {
  const currency = data.currency || "INR";
  const fmt = new Intl.NumberFormat("en-IN", {
    style: "currency",
    currency,
    maximumFractionDigits: 2,
  });

  // Compute totals client-side from items — more reliable than asking the LLM.
  const totals: Record<string, number> = {};
  for (const it of data.items ?? []) {
    totals[it.category] = (totals[it.category] ?? 0) + (it.amount || 0);
  }
  const totalEntries = Object.entries(totals).sort((a, b) => b[1] - a[1]);

  return (
    <div className="space-y-5">
      <div className="flex items-baseline justify-between border-b border-[var(--border)] pb-3">
        <div>
          <div className="text-xs uppercase tracking-wider text-[var(--text-dim)]">
            Expenses
          </div>
          <div className="text-3xl font-semibold tabular-nums mt-1">
            {fmt.format(data.grand_total ?? 0)}
          </div>
        </div>
        <div className="text-xs text-[var(--text-dim)]">
          {data.items?.length ?? 0} item{(data.items?.length ?? 0) === 1 ? "" : "s"}
        </div>
      </div>

      {totalEntries.length > 0 && (
        <div className="space-y-1.5">
          {totalEntries.map(([cat, amt]) => {
            const pct =
              data.grand_total > 0 ? (amt / data.grand_total) * 100 : 0;
            return (
              <div key={cat}>
                <div className="flex justify-between text-sm">
                  <span className="text-[var(--text-dim)]">{cat}</span>
                  <span className="tabular-nums">{fmt.format(amt)}</span>
                </div>
                <div className="h-1 rounded-full bg-[var(--panel-2)] overflow-hidden mt-1">
                  <div
                    className="h-full bg-[var(--accent)]"
                    style={{ width: `${pct}%` }}
                  />
                </div>
              </div>
            );
          })}
        </div>
      )}

      <div className="overflow-hidden rounded-lg border border-[var(--border)]">
        <table className="w-full text-sm">
          <thead className="bg-[var(--panel-2)] text-[var(--text-dim)]">
            <tr>
              <th className="text-left px-3 py-2 font-medium">Date</th>
              <th className="text-left px-3 py-2 font-medium">Category</th>
              <th className="text-left px-3 py-2 font-medium">Merchant</th>
              <th className="text-right px-3 py-2 font-medium">Amount</th>
            </tr>
          </thead>
          <tbody>
            {(data.items ?? []).map((it, i) => (
              <tr
                key={i}
                className="border-t border-[var(--border)] hover:bg-[var(--panel-2)]/60"
              >
                <td className="px-3 py-2 text-[var(--text-dim)]">{it.date || "—"}</td>
                <td className="px-3 py-2">{it.category}</td>
                <td className="px-3 py-2">{it.merchant || it.note || "—"}</td>
                <td className="px-3 py-2 text-right tabular-nums">
                  {fmt.format(it.amount)}
                </td>
              </tr>
            ))}
            {(data.items ?? []).length === 0 && (
              <tr>
                <td colSpan={4} className="px-3 py-6 text-center text-[var(--text-dim)]">
                  No items extracted.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {(data.flags?.length ?? 0) > 0 && (
        <div className="rounded-md border border-yellow-800/50 bg-yellow-950/30 px-3 py-2 text-xs text-yellow-200/90">
          <div className="font-medium mb-1">Flags</div>
          <ul className="list-disc list-inside space-y-0.5">
            {data.flags!.map((f, i) => (
              <li key={i}>{f}</li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
