import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { JobsTable } from "@/components/jobs-table";
import { Button } from "@/components/ui/button";

const FILTERS = ["All", "Queued", "Running", "Succeeded", "Failed", "Stopped"];

export function JobsPage() {
  const [filter, setFilter] = useState("All");
  const status = filter === "All" ? undefined : filter;

  const jobs = useQuery({
    queryKey: ["jobs", status ?? ""],
    queryFn: () => api.jobs(status),
    refetchInterval: 15000,
  });

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Jobs</h1>
        <p className="text-sm text-muted-foreground">
          Queued, running, and recently completed jobs.
        </p>
      </div>

      <div className="flex flex-wrap gap-2">
        {FILTERS.map((f) => (
          <Button
            key={f}
            variant={filter === f ? "default" : "outline"}
            size="sm"
            onClick={() => setFilter(f)}
          >
            {f}
          </Button>
        ))}
      </div>

      <JobsTable jobs={jobs.data} loading={jobs.isLoading} />
    </div>
  );
}
