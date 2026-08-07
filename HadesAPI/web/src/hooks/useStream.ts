import { useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import type { JobSummary, Metrics } from "@/lib/types";

type StreamEvent =
  | { type: "job"; job: JobSummary }
  | { type: "metrics"; metrics: Metrics };

/**
 * useStream subscribes to the /api/stream SSE feed and pushes live updates into
 * the React Query cache: job events patch the jobs list, metrics events replace
 * the metrics snapshot. Returns whether the stream is currently connected.
 */
export function useStream(enabled: boolean): boolean {
  const qc = useQueryClient();
  const [connected, setConnected] = useState(false);
  const esRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (!enabled) return;

    const es = new EventSource("/api/stream", { withCredentials: true });
    esRef.current = es;

    es.onopen = () => setConnected(true);
    es.onerror = () => setConnected(false);

    es.onmessage = (e) => {
      let data: StreamEvent;
      try {
        data = JSON.parse(e.data);
      } catch {
        return;
      }
      if (data.type === "metrics") {
        qc.setQueryData(["metrics"], data.metrics);
      } else if (data.type === "job" && data.job?.id) {
        patchJob(qc, data.job);
      }
    };

    return () => {
      es.close();
      esRef.current = null;
      setConnected(false);
    };
  }, [enabled, qc]);

  return connected;
}

function patchJob(
  qc: ReturnType<typeof useQueryClient>,
  job: JobSummary,
): void {
  // Update every cached jobs list (filtered variants included).
  qc.setQueriesData<JobSummary[]>({ queryKey: ["jobs"] }, (prev) => {
    const list = prev ?? [];
    const idx = list.findIndex((j) => j.id === job.id);
    const merged =
      idx >= 0
        ? Object.assign([...list], { [idx]: { ...list[idx], ...job } })
        : [job, ...list];
    return merged as JobSummary[];
  });
  // Merge into an open detail view if present.
  qc.setQueryData(["job", job.id], (prev: unknown) =>
    prev ? { ...(prev as object), ...job } : prev,
  );
}
