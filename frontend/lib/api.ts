import type { AnalysisResult, BlockType } from "./types";

const API_URL =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// analyzeStream POSTs the note text to the backend and subscribes to SSE events.
// Invokes callbacks as events arrive. Returns an abort function.
export function analyzeStream(
  text: string,
  opts: {
    onClassified?: (type: BlockType, confidence: number) => void;
    onDone?: (result: AnalysisResult) => void;
    onError?: (message: string) => void;
  },
  signal?: AbortSignal
): Promise<void> {
  return (async () => {
    let res: Response;
    try {
      res = await fetch(`${API_URL}/api/analyze/stream`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "text/event-stream",
        },
        body: JSON.stringify({ text }),
        signal,
      });
    } catch (err: unknown) {
      if ((err as Error)?.name === "AbortError") return;
      opts.onError?.((err as Error)?.message ?? "network error");
      return;
    }

    if (!res.ok || !res.body) {
      const text = await res.text().catch(() => "");
      opts.onError?.(text || `HTTP ${res.status}`);
      return;
    }

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = "";

    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });

        // SSE frames are separated by \n\n
        let idx: number;
        while ((idx = buf.indexOf("\n\n")) !== -1) {
          const frame = buf.slice(0, idx);
          buf = buf.slice(idx + 2);
          handleFrame(frame, opts);
        }
      }
    } catch (err: unknown) {
      if ((err as Error)?.name === "AbortError") return;
      opts.onError?.((err as Error)?.message ?? "stream error");
    }
  })();
}

function handleFrame(
  frame: string,
  opts: {
    onClassified?: (type: BlockType, confidence: number) => void;
    onDone?: (result: AnalysisResult) => void;
    onError?: (message: string) => void;
  }
) {
  let event = "message";
  let dataLines: string[] = [];
  for (const line of frame.split("\n")) {
    if (line.startsWith("event:")) event = line.slice(6).trim();
    else if (line.startsWith("data:")) dataLines.push(line.slice(5).trim());
  }
  const data = dataLines.join("\n");
  if (!data) return;

  try {
    const payload = JSON.parse(data);
    if (event === "classified") {
      opts.onClassified?.(payload.type, payload.confidence);
    } else if (event === "done") {
      opts.onDone?.(payload as AnalysisResult);
    } else if (event === "error") {
      opts.onError?.(payload.message ?? "unknown error");
    }
  } catch {
    // ignore malformed frames
  }
}
