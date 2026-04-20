import type { GenericStructured } from "@/lib/types";

interface Props {
  data: GenericStructured;
}

export function GenericView({ data }: Props) {
  return (
    <div className="space-y-5">
      <div className="border-b border-[var(--border)] pb-3">
        <div className="text-xs uppercase tracking-wider text-[var(--text-dim)]">
          Summary
        </div>
        <p className="mt-1 text-lg leading-relaxed">{data.summary}</p>
      </div>

      {(data.themes?.length ?? 0) > 0 && (
        <Section title="Themes">
          <div className="flex flex-wrap gap-2">
            {data.themes!.map((t, i) => (
              <span
                key={i}
                className="text-xs px-2 py-1 rounded-full border border-[var(--border)] bg-[var(--panel-2)]"
              >
                {t}
              </span>
            ))}
          </div>
        </Section>
      )}

      {(data.questions?.length ?? 0) > 0 && (
        <Section title="Open questions">
          <ul className="list-disc list-inside space-y-1 text-sm">
            {data.questions!.map((q, i) => (
              <li key={i}>{q}</li>
            ))}
          </ul>
        </Section>
      )}

      {(data.action_candidates?.length ?? 0) > 0 && (
        <Section title="Action candidates">
          <ul className="space-y-1.5 text-sm">
            {data.action_candidates!.map((a, i) => (
              <li
                key={i}
                className="rounded-md border border-[var(--border)] bg-[var(--panel-2)] px-3 py-1.5"
              >
                {a}
              </li>
            ))}
          </ul>
        </Section>
      )}
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="text-xs uppercase tracking-wider text-[var(--text-dim)] mb-2">
        {title}
      </div>
      {children}
    </div>
  );
}
