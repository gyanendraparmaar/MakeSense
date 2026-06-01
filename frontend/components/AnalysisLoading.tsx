"use client";

import { useEffect, useState } from "react";
import type { BlockType } from "@/lib/types";

interface Props {
  phase: "classifying" | "analyzing";
  type?: BlockType;
}

const CLASSIFYING_MESSAGES = [
  "Reading your text…",
  "Detecting what you're writing…",
  "Figuring out the format…",
];

const ANALYZING_MESSAGES: Record<BlockType | "default", string[]> = {
  expenses: [
    "Extracting amounts and categories…",
    "Calculating totals…",
    "Organizing your expenses…",
  ],
  todo: [
    "Finding tasks and deadlines…",
    "Sorting by priority…",
    "Building your to-do list…",
  ],
  generic: [
    "Summarizing key points…",
    "Structuring the content…",
    "Polishing the output…",
  ],
  default: [
    "Extracting details…",
    "Organizing the structure…",
    "Almost there…",
  ],
};

export function AnalysisLoading({ phase, type }: Props) {
  const messages =
    phase === "classifying"
      ? CLASSIFYING_MESSAGES
      : ANALYZING_MESSAGES[type ?? "default"] ?? ANALYZING_MESSAGES.default;

  const [index, setIndex] = useState(0);
  const [visible, setVisible] = useState(true);

  useEffect(() => {
    setIndex(0);
    setVisible(true);
  }, [phase, type]);

  useEffect(() => {
    const interval = setInterval(() => {
      setVisible(false);
      setTimeout(() => {
        setIndex((i) => (i + 1) % messages.length);
        setVisible(true);
      }, 280);
    }, 2400);
    return () => clearInterval(interval);
  }, [messages.length]);

  return (
    <div className="h-full flex flex-col items-center justify-center px-8 py-12">
      <div className="flex flex-col items-center gap-6 max-w-sm w-full">
        <ThinkingIndicator />

        <div className="text-center min-h-[3rem] flex items-center justify-center">
          <p
            className={
              "text-sm text-[var(--text-dim)] transition-all duration-300 " +
              (visible ? "opacity-100 translate-y-0" : "opacity-0 translate-y-1")
            }
          >
            {messages[index]}
          </p>
        </div>

        <div className="w-full space-y-2.5 mt-2">
          <ShimmerBar width="72%" delay={0} />
          <ShimmerBar width="100%" delay={120} />
          <ShimmerBar width="88%" delay={240} />
          <ShimmerBar width="60%" delay={360} />
        </div>
      </div>
    </div>
  );
}

function ThinkingIndicator() {
  return (
    <div className="relative flex items-center justify-center h-14 w-14">
      <span className="absolute inset-0 rounded-full bg-[var(--accent)]/10 animate-ping" />
      <span className="absolute inset-1 rounded-full bg-[var(--accent)]/5 animate-pulse" />
      <span className="relative flex items-center justify-center h-10 w-10 rounded-full bg-[var(--panel-2)] border border-[var(--border)]">
        <span className="flex gap-1">
          <Dot delay={0} />
          <Dot delay={160} />
          <Dot delay={320} />
        </span>
      </span>
    </div>
  );
}

function Dot({ delay }: { delay: number }) {
  return (
    <span
      className="inline-block h-1.5 w-1.5 rounded-full bg-[var(--accent)] animate-bounce"
      style={{ animationDelay: `${delay}ms`, animationDuration: "1s" }}
    />
  );
}

function ShimmerBar({ width, delay }: { width: string; delay: number }) {
  return (
    <div
      className="h-3 rounded-md bg-[var(--panel-2)] overflow-hidden relative"
      style={{ width }}
    >
      <div
        className="absolute inset-0 -translate-x-full animate-shimmer bg-gradient-to-r from-transparent via-white/[0.06] to-transparent"
        style={{ animationDelay: `${delay}ms` }}
      />
    </div>
  );
}
