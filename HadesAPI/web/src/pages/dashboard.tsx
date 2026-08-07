import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "@/lib/api";
import { MetricsCards } from "@/components/metrics-cards";
import { JobsTable } from "@/components/jobs-table";
import { Button } from "@/components/ui/button";

export function DashboardPage() {
  const metrics = useQuery({
    queryKey: ["metrics"],
    queryFn: api.metrics,
    refetchInterval: 15000,
  });

  const jobs = useQuery({
    queryKey: ["jobs"],
    queryFn: () => api.jobs(),
    refetchInterval: 15000,
  });

  const recent = (jobs.data ?? []).slice(0, 8);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Overview</h1>
        <p className="text-sm text-muted-foreground">
          Live system metrics and recent activity.
        </p>
      </div>

      <MetricsCards metrics={metrics.data} />

      <section className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-medium">Recent jobs</h2>
          <Button variant="outline" size="sm" asChild>
            <Link to="/jobs">View all</Link>
          </Button>
        </div>
        <JobsTable jobs={recent} loading={jobs.isLoading} />
      </section>
    </div>
  );
}
