import {
  Bar,
  BarChart,
  Cell,
  ResponsiveContainer,
  Tooltip,
  XAxis,
} from "recharts";

// Recharts is heavy (~hundreds of KB), so this chart is loaded lazily and only
// pulled into the bundle on the pages that render it (not the login page).

const STATUS_COLOR: Record<string, string> = {
  Queued: "var(--color-muted-foreground)",
  Running: "var(--color-info)",
  Succeeded: "var(--color-success)",
  Failed: "var(--color-destructive)",
  Stopped: "var(--color-warning)",
};

export interface StatusDatum {
  status: string;
  count: number;
}

export default function StatusChart({ data }: { data: StatusDatum[] }) {
  return (
    <ResponsiveContainer width="100%" height={180}>
      <BarChart data={data}>
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
          {data.map((d) => (
            <Cell
              key={d.status}
              fill={STATUS_COLOR[d.status] ?? "var(--color-primary)"}
            />
          ))}
        </Bar>
      </BarChart>
    </ResponsiveContainer>
  );
}
