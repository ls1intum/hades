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

// maxListLength caps how many jobs a live-patched list holds, so a busy cluster
// cannot grow a tab's memory without bound between the periodic full refetches.
const MAX_LIST_LENGTH = 500;

export function patchJob(
  qc: ReturnType<typeof useQueryClient>,
  job: JobSummary,
): void {
  // Update each cached jobs list, respecting its status filter. The query key is
  // ["jobs", filter] where filter is "" (all) or a specific status.
  const queries = qc.getQueryCache().findAll({ queryKey: ["jobs"] });
  for (const query of queries) {
    const filter = (query.queryKey[1] as string | undefined) ?? "";
    const matches = filter === "" || job.status === filter;
    qc.setQueryData<JobSummary[]>(query.queryKey, (prev) => {
      const list = prev ?? [];
      const idx = list.findIndex((j) => j.id === job.id);
      if (!matches) {
        // Job no longer belongs in this filtered list: drop it if present.
        return idx >= 0 ? list.filter((j) => j.id !== job.id) : list;
      }
      if (idx >= 0) {
        const next = [...list];
        next[idx] = { ...list[idx], ...job };
        return next;
      }
      return [job, ...list].slice(0, MAX_LIST_LENGTH);
    });
  }
  // Merge into an open detail view if present.
  qc.setQueryData(["job", job.id], (prev: unknown) =>
    prev ? { ...(prev as object), ...job } : prev,
  );
}
