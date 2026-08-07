import { useQuery } from "@tanstack/react-query";
import { AlertTriangle } from "lucide-react";
import { api } from "@/lib/api";
import { Skeleton } from "@/components/ui/skeleton";
import type { JobStatus } from "@/lib/types";

/**
 * LogsViewer streams a job's aggregated logs. While the job is active it polls;
 * logs are shown verbatim - a banner reminds operators they are not scrubbed.
 */
export function LogsViewer({
  jobId,
  status,
}: {
  jobId: string;
  status: JobStatus;
}) {
  const active = status === "Running" || status === "Queued";
  const logs = useQuery({
    queryKey: ["logs", jobId],
    queryFn: () => api.logs(jobId),
    refetchInterval: active ? 3000 : false,
  });

  if (logs.isLoading) return <Skeleton className="h-64 w-full" />;

  if (logs.isError) {
    return (
      <div className="rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground">
        Logs are currently unavailable.
      </div>
    );
  }

  const groups = logs.data?.logs ?? [];
  const lineCount = groups.reduce((n, g) => n + (g.logs?.length ?? 0), 0);

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2 rounded-md border border-[var(--color-warning)]/40 bg-[var(--color-warning)]/10 px-3 py-2 text-xs text-muted-foreground">
        <AlertTriangle className="size-4 shrink-0 text-[var(--color-warning)]" />
        Logs are shown verbatim and may contain secrets Hades does not scrub.
      </div>

      {lineCount === 0 ? (
        <div className="rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground">
          {active ? "Waiting for log output..." : "No logs recorded for this job."}
        </div>
      ) : (
        <div className="max-h-[28rem] overflow-auto rounded-md bg-black/90 p-4 font-mono text-xs leading-relaxed text-green-200">
          {groups.map((group) => (
            <div key={group.container_id} className="mb-4">
              <div className="mb-1 text-[10px] uppercase tracking-wide text-green-500/70">
                container {group.container_id.slice(0, 12)}
              </div>
              {group.logs.map((entry, i) => (
                <div key={i} className="whitespace-pre-wrap break-all">
                  <span className="text-green-500/50">
                    {new Date(entry.timestamp).toLocaleTimeString()}{" "}
                  </span>
                  <span
                    className={
                      entry.output_stream === "stderr"
                        ? "text-red-300"
                        : undefined
                    }
                  >
                    {entry.message}
                  </span>
                </div>
              ))}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
