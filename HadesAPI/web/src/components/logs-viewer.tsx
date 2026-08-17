import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle } from "lucide-react";
import { api } from "@/lib/api";
import { useJobLogStream } from "@/hooks/useJobLogStream";
import { Skeleton } from "@/components/ui/skeleton";
import type { JobStatus } from "@/lib/types";

// Cap rendered lines so a job that logged megabytes doesn't blow up the DOM.
const MAX_LINES = 2000;

interface FlatLine {
  key: string;
  container: string;
  time: string;
  message: string;
  stderr: boolean;
}

/**
 * LogsViewer shows a job's aggregated logs. While the job is active it subscribes
 * to the live SSE log stream (which replays the backlog then tails); once the job
 * is terminal it falls back to a one-shot snapshot fetch. Logs are shown verbatim
 * - a banner reminds operators they are not scrubbed. Lines are flattened/
 * formatted (memoized) and capped to the most recent MAX_LINES.
 */
export function LogsViewer({
  jobId,
  status,
}: {
  jobId: string;
  status: JobStatus;
}) {
  const active = status === "Running" || status === "Queued";
  // Active jobs are driven by the live SSE stream (populates the ["logs", jobId]
  // cache); terminal jobs fetch a single snapshot from the log manager.
  useJobLogStream(jobId, active);
  const logs = useQuery({
    queryKey: ["logs", jobId],
    queryFn: () => api.logs(jobId),
    enabled: !active,
    refetchInterval: false,
  });

  const { lines, total } = useMemo(() => {
    const groups = logs.data?.logs ?? [];
    const all: FlatLine[] = [];
    for (const group of groups) {
      const container = group.container_id.slice(0, 12);
      group.logs.forEach((entry, i) => {
        all.push({
          key: `${group.container_id}:${i}`,
          container,
          time: new Date(entry.timestamp).toLocaleTimeString(),
          message: entry.message,
          stderr: entry.output_stream === "stderr",
        });
      });
    }
    return { lines: all.slice(-MAX_LINES), total: all.length };
  }, [logs.data]);

  if (logs.isLoading) return <Skeleton className="min-h-0 w-full flex-1" />;

  if (logs.isError) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground">
        Logs are currently unavailable.
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <div className="flex items-center gap-2 rounded-md border border-[var(--color-warning)]/40 bg-[var(--color-warning)]/10 px-3 py-2 text-xs text-muted-foreground">
        <AlertTriangle className="size-4 shrink-0 text-[var(--color-warning)]" />
        Logs are shown verbatim and may contain secrets Hades does not scrub.
      </div>

      {total > MAX_LINES && (
        <p className="text-xs text-muted-foreground">
          Showing the most recent {MAX_LINES} of {total} lines.
        </p>
      )}

      {total === 0 ? (
        <div className="flex min-h-0 flex-1 items-center justify-center rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground">
          {active ? "Waiting for log output..." : "No logs recorded for this job."}
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-auto rounded-md bg-black/90 p-4 font-mono text-xs leading-relaxed text-green-200">
          {lines.map((line) => (
            <div key={line.key} className="whitespace-pre-wrap break-all">
              <span className="text-green-500/50">
                {line.container} {line.time}{" "}
              </span>
              <span className={line.stderr ? "text-red-300" : undefined}>
                {line.message}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
