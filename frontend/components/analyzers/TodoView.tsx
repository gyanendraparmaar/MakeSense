"use client";

import { useState } from "react";
import type { TodoStructured, TodoPriority } from "@/lib/types";

interface Props {
  data: TodoStructured;
}

const priorityStyles: Record<TodoPriority, string> = {
  high: "bg-red-500/15 text-red-300 border border-red-500/30",
  medium: "bg-yellow-500/10 text-yellow-300 border border-yellow-500/30",
  low: "bg-emerald-500/10 text-emerald-300 border border-emerald-500/30",
};

export function TodoView({ data }: Props) {
  // Local optimistic toggle so the user can check things off.
  // This is pure UI state; not persisted.
  const [checked, setChecked] = useState<Record<number, boolean>>({});

  const items = data.items ?? [];
  const doneCount =
    items.filter((it, i) => (checked[i] ?? it.done) === true).length;

  return (
    <div className="space-y-4">
      <div className="flex items-baseline justify-between border-b border-[var(--border)] pb-3">
        <div>
          <div className="text-xs uppercase tracking-wider text-[var(--text-dim)]">
            To-do
          </div>
          <div className="text-3xl font-semibold mt-1">
            {doneCount}{" "}
            <span className="text-[var(--text-dim)] text-xl font-normal">
              / {items.length} done
            </span>
          </div>
        </div>
      </div>

      <ul className="space-y-2">
        {items.map((it, i) => {
          const isDone = checked[i] ?? it.done ?? false;
          return (
            <li
              key={i}
              className="flex items-start gap-3 rounded-md border border-[var(--border)] bg-[var(--panel-2)] px-3 py-2"
            >
              <button
                type="button"
                aria-label="toggle done"
                onClick={() =>
                  setChecked((prev) => ({ ...prev, [i]: !isDone }))
                }
                className={
                  "mt-0.5 h-5 w-5 shrink-0 rounded border transition-colors " +
                  (isDone
                    ? "bg-[var(--accent)] border-[var(--accent)]"
                    : "border-[var(--border)] hover:border-[var(--accent-dim)]")
                }
              >
                {isDone && (
                  <svg
                    viewBox="0 0 24 24"
                    className="h-full w-full text-[#0b0c0f]"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="3"
                  >
                    <path d="M5 12l5 5L20 7" strokeLinecap="round" strokeLinejoin="round" />
                  </svg>
                )}
              </button>

              <div className="flex-1 min-w-0">
                <div
                  className={
                    "text-sm " +
                    (isDone
                      ? "line-through text-[var(--text-dim)]"
                      : "")
                  }
                >
                  {it.task}
                </div>
                <div className="mt-1 flex items-center gap-2 text-xs text-[var(--text-dim)]">
                  <span
                    className={
                      "px-1.5 py-0.5 rounded-md text-[10px] uppercase tracking-wide font-medium " +
                      priorityStyles[it.priority]
                    }
                  >
                    {it.priority}
                  </span>
                  {it.due && <span>• due {it.due}</span>}
                  {it.depends_on && (
                    <span>• after <em>{it.depends_on}</em></span>
                  )}
                </div>
              </div>
            </li>
          );
        })}
        {items.length === 0 && (
          <li className="text-sm text-[var(--text-dim)]">No tasks found.</li>
        )}
      </ul>
    </div>
  );
}
