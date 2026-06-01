"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Editor } from "@/components/Editor";
import { AnalysisPane } from "@/components/AnalysisPane";
import { analyzeStream } from "@/lib/api";
import type { AnalysisResult, BlockType } from "@/lib/types";

const DEBOUNCE_MS = 900;
const MIN_CHARS = 15;

export default function HomePage() {
  const [text, setText] = useState("");
  const [status, setStatus] = useState<
    "idle" | "classifying" | "analyzing" | "ready" | "error"
  >("idle");
  const [type, setType] = useState<BlockType | undefined>(undefined);
  const [confidence, setConfidence] = useState<number | undefined>(undefined);
  const [result, setResult] = useState<AnalysisResult | undefined>(undefined);
  const [errorMsg, setErrorMsg] = useState<string | undefined>(undefined);
  const [cached, setCached] = useState(false);

  // Keep track of the in-flight request so we can abort on new edits.
  const abortRef = useRef<AbortController | null>(null);
  // Debounce timer
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const runAnalysis = useCallback((snapshot: string) => {
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;

    setStatus("classifying");
    setErrorMsg(undefined);
    setCached(false);

    void analyzeStream(
      snapshot,
      {
        onClassified: (t, c) => {
          setType(t);
          setConfidence(c);
          setStatus("analyzing");
        },
        onDone: (r) => {
          setResult(r);
          setType(r.type);
          setConfidence(r.confidence);
          setCached(!!r.cached);
          setStatus("ready");
        },
        onError: (msg) => {
          setErrorMsg(msg);
          setStatus("error");
        },
      },
      ctrl.signal
    );
  }, []);

  const handleChange = useCallback(
    (plain: string) => {
      setText(plain);
      if (timerRef.current) clearTimeout(timerRef.current);

      const trimmed = plain.trim();
      if (trimmed.length < MIN_CHARS) {
        setStatus("idle");
        setResult(undefined);
        abortRef.current?.abort();
        return;
      }

      timerRef.current = setTimeout(() => {
        runAnalysis(trimmed);
      }, DEBOUNCE_MS);
    },
    [runAnalysis]
  );

  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
      abortRef.current?.abort();
    };
  }, []);

  return (
    <main className="h-screen w-screen flex flex-col">
      {/* Top bar */}
      <header className="flex items-center justify-between px-6 py-3 border-b border-[var(--border)] bg-[var(--panel)]">
        <div className="flex items-center gap-2">
          <span className="text-lg font-semibold tracking-tight">
            MakeSense<span className="text-[var(--accent)]">.ai</span>
          </span>
          <span className="text-[11px] text-[var(--text-dim)]">
            write anything. see it organized.
          </span>
        </div>
        <div className="text-[11px] text-[var(--text-dim)]">
          {text.trim().split(/\s+/).filter(Boolean).length} words
        </div>
      </header>

      {/* Split pane */}
      <div className="flex-1 grid grid-cols-1 md:grid-cols-2 min-h-0">
        <section className="min-h-0 border-r border-[var(--border)] bg-[var(--panel)]">
          <Editor onChange={handleChange} />
        </section>
        <section className="min-h-0 h-full bg-[var(--bg)]">
          <AnalysisPane
            status={status}
            type={type}
            confidence={confidence}
            result={result}
            errorMessage={errorMsg}
            cached={cached}
          />
        </section>
      </div>
    </main>
  );
}
