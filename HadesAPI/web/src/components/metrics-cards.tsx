import {
  Bar,
  BarChart,
  Cell,
  ResponsiveContainer,
  Tooltip,
  XAxis,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import type { Metrics } from "@/lib/types";
import { formatDuration } from "@/lib/utils";

const STATUS_ORDER = ["Queued", "Running", "Succeeded", "Failed", "Stopped"];
const STATUS_COLOR: Record<string, string> = {
  Queued: "var(--color-muted-foreground)",
  Running: "var(--color-info)",
  Succeeded: "var(--color-success)",
  Failed: "var(--color-destructive)",
  Stopped: "var(--color-warning)",
};

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

export function MetricsCards({ metrics }: { metrics?: Metrics }) {
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
          <ResponsiveContainer width="100%" height={180}>
            <BarChart data={chartData}>
              <XAxis
                dataKey="status"
                tickLine={false}
                axisLine={false}
                fontSize={12}
                stroke="var(--color-muted-foreground)"
              />
              <Tooltip
                cursor={{ fill: "var(--color-muted)", opacity: 0.3 }}
                contentStyle={{
                  background: "var(--color-popover)",
                  border: "1px solid var(--color-border)",
                  borderRadius: 8,
                  color: "var(--color-popover-foreground)",
                  fontSize: 12,
                }}
              />
              <Bar dataKey="count" radius={[4, 4, 0, 0]}>
                {chartData.map((d) => (
                  <Cell
                    key={d.status}
                    fill={STATUS_COLOR[d.status] ?? "var(--color-primary)"}
                  />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </CardContent>
      </Card>
    </div>
  );
}
