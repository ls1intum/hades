import { lazy, Suspense } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import type { Metrics } from "@/lib/types";
import { formatDuration } from "@/lib/utils";

// Lazy so Recharts is not in the initial/login bundle.
const StatusChart = lazy(() => import("@/components/status-chart"));

const STATUS_ORDER = ["Queued", "Running", "Succeeded", "Failed", "Stopped"];

function Stat({ label, value, hint }: { label: string; value: React.ReactNode; hint?: string }) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          {label}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-semibold tabular-nums">{value}</div>
        {hint && <p className="mt-1 text-xs text-muted-foreground">{hint}</p>}
      </CardContent>
    </Card>
  );
}

export function MetricsCards({
  metrics,
  error,
}: {
  metrics?: Metrics;
  error?: boolean;
}) {
  if (error && !metrics) {
    return (
      <div
        role="alert"
        className="rounded-xl border border-destructive/40 bg-destructive/5 p-6 text-sm text-destructive"
      >
        Failed to load metrics. Retrying…
      </div>
    );
  }
  if (!metrics) {
    return (
      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-28" />
        ))}
      </div>
    );
  }

  const counts = metrics.statusCounts ?? {};
  const running = counts["Running"] ?? 0;
  const queueTotal = metrics.queueDepth?.total ?? 0;

  const chartData = STATUS_ORDER.map((s) => ({
    status: s,
    count: counts[s] ?? 0,
  }));

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <Stat
          label="Queued"
          value={queueTotal}
          hint={metrics.queueDepth?.approximate ? "approximate" : undefined}
        />
        <Stat label="Running" value={running} />
        <Stat label="Throughput" value={`${metrics.throughputPerMin}/min`} />
        <Stat
          label="Avg duration"
          value={formatDuration(metrics.durations?.avgMs)}
          hint={`p95 ${formatDuration(metrics.durations?.p95Ms)} · n=${metrics.durations?.count ?? 0}`}
        />
      </div>

      <Card>
        <CardHeader className="pb-0">
          <CardTitle className="text-sm font-medium text-muted-foreground">
            Jobs by status
          </CardTitle>
        </CardHeader>
        <CardContent className="pt-4">
          <Suspense fallback={<Skeleton className="h-[180px] w-full" />}>
            <StatusChart data={chartData} />
          </Suspense>
        </CardContent>
      </Card>
    </div>
  );
}
