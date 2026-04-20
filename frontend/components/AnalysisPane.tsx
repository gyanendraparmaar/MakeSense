"use client";

import { ExpensesView } from "./analyzers/ExpensesView";
import { TodoView } from "./analyzers/TodoView";
import { GenericView } from "./analyzers/GenericView";
import type {
  AnalysisResult,
  BlockType,
  ExpensesStructured,
  GenericStructured,
  TodoStructured,
} from "@/lib/types";

interface Props {
  status: "idle" | "classifying" | "analyzing" | "ready" | "error";
  type?: BlockType;
  confidence?: number;
  result?: AnalysisResult;
  errorMessage?: string;
  cached?: boolean;
}

export function AnalysisPane(props: Props) {
  return (
    <div className="h-full flex flex-col">
      <Header {...props} />
      <div className="flex-1 overflow-y-auto px-6 py-5">
        <Body {...props} />
      </div>
    </div>
  );
}

function Header({ status, type, confidence, cached }: Props) {
  const label =
    status === "idle"
      ? "Waiting for input"
      : status === "classifying"
      ? "Detecting type…"
      : status === "analyzing"
      ? `Analyzing (${type})…`
      : status === "error"
      ? "Error"
      : type
      ? labelFor(type)
      : "";

  return (
    <div className="flex items-center justify-between px-6 py-3 border-b border-[var(--border)]">
      <div className="flex items-center gap-2">
        <span
          className={
            "inline-block h-2 w-2 rounded-full " +
            (status === "error"
              ? "bg-red-400"
              : status === "ready"
              ? "bg-emerald-400"
              : status === "idle"
              ? "bg-[var(--text-dim)]"
              : "bg-amber-400 animate-pulse")
          }
        />
        <span className="text-sm font-medium">{label}</span>
        {confidence != null && status === "ready" && (
          <span className="text-[11px] text-[var(--text-dim)]">
            {(confidence * 100).toFixed(0)}% confidence
          </span>
        )}
      </div>
      <div className="text-[11px] text-[var(--text-dim)] uppercase tracking-wider">
        {cached ? "cached" : "live"}
      </div>
    </div>
  );
}

function labelFor(t: BlockType) {
  switch (t) {
    case "expenses":
      return "Expenses detected";
    case "todo":
      return "To-do list detected";
    default:
      return "Summary";
  }
}

function Body({ status, result, errorMessage }: Props) {
  if (status === "idle") {
    return (
      <EmptyState>
        Start typing on the left. As soon as you pause, MakeSense will detect
        what you&apos;re writing and structure it here.
      </EmptyState>
    );
  }
  if (status === "error") {
    return (
      <div className="rounded-md border border-red-800/50 bg-red-950/30 px-4 py-3 text-sm text-red-200">
        {errorMessage || "Something went wrong."}
      </div>
    );
  }
  if (status === "classifying" || status === "analyzing") {
    return <Skeleton />;
  }
  if (!result) return null;

  switch (result.type) {
    case "expenses":
      return <ExpensesView data={result.structured as ExpensesStructured} />;
    case "todo":
      return <TodoView data={result.structured as TodoStructured} />;
    default:
      return <GenericView data={result.structured as GenericStructured} />;
  }
}

function EmptyState({ children }: { children: React.ReactNode }) {
  return (
    <div className="h-full flex items-center justify-center text-center px-8">
      <p className="max-w-sm text-sm text-[var(--text-dim)] leading-relaxed">
        {children}
      </p>
    </div>
  );
}

function Skeleton() {
  return (
    <div className="space-y-3 animate-pulse">
      <div className="h-5 w-40 rounded bg-[var(--panel-2)]" />
      <div className="h-9 w-56 rounded bg-[var(--panel-2)]" />
      <div className="h-32 w-full rounded bg-[var(--panel-2)]" />
      <div className="h-32 w-full rounded bg-[var(--panel-2)]" />
    </div>
  );
}
