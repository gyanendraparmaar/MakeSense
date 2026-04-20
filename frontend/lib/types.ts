// Shape of /api/analyze response and SSE 'done' payloads.
export type BlockType = "expenses" | "todo" | "generic";

export interface AnalysisResult {
  type: BlockType;
  confidence: number;
  structured: unknown; // type-narrowed per BlockType below
  model: string;
  cached?: boolean;
}

// --- Expenses ---

export interface ExpenseItem {
  date?: string;
  category: string;
  merchant?: string;
  amount: number;
  note?: string;
}

export interface ExpensesStructured {
  currency: string;
  items: ExpenseItem[];
  grand_total: number;
  flags?: string[];
}

// --- Todo ---

export type TodoPriority = "high" | "medium" | "low";

export interface TodoItem {
  task: string;
  due?: string;
  priority: TodoPriority;
  depends_on?: string;
  done?: boolean;
}

export interface TodoStructured {
  items: TodoItem[];
}

// --- Generic summary ---

export interface GenericStructured {
  summary: string;
  themes?: string[];
  questions?: string[];
  action_candidates?: string[];
}
