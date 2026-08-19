import { Link, useNavigate } from "react-router-dom";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusBadge } from "@/components/status-badge";
import type { JobSummary } from "@/lib/types";
import { formatDuration, relativeTime } from "@/lib/utils";

export function JobsTable({
  jobs,
  loading,
  error,
}: {
  jobs?: JobSummary[];
  loading?: boolean;
  error?: boolean;
}) {
  const navigate = useNavigate();

  if (loading) {
    return <Skeleton className="h-64 w-full" />;
  }
  if (error) {
    return (
      <div
        role="alert"
        className="rounded-lg border border-destructive/40 bg-destructive/5 p-10 text-center text-sm text-destructive"
      >
        Failed to load jobs. Retrying…
      </div>
    );
  }
  if (!jobs || jobs.length === 0) {
    return (
      <div className="rounded-lg border border-dashed p-10 text-center text-sm text-muted-foreground">
        No jobs in the current window.
      </div>
    );
  }

  return (
    <div className="rounded-lg border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Status</TableHead>
            <TableHead className="hidden sm:table-cell">Priority</TableHead>
            <TableHead className="hidden md:table-cell">Steps</TableHead>
            <TableHead className="hidden md:table-cell">Duration</TableHead>
            <TableHead className="text-right">Updated</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {jobs.map((job) => (
            <TableRow
              key={job.id}
              className="cursor-pointer"
              onClick={() => navigate(`/jobs/${job.id}`)}
            >
              <TableCell className="font-medium">
                {/* Real link so the row is keyboard/AT navigable while the <tr>
                    keeps its table-row semantics; the row onClick is a
                    mouse-only convenience. */}
                <Link
                  to={`/jobs/${job.id}`}
                  aria-label={`Open job ${job.name || job.id}`}
                  onClick={(e) => e.stopPropagation()}
                  className="rounded hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                  {job.name || "(unnamed)"}
                </Link>
                <div className="font-mono text-xs text-muted-foreground">
                  {job.id.slice(0, 8)}
                </div>
              </TableCell>
              <TableCell>
                <StatusBadge status={job.status} />
                {job.reason && (
                  <div
                    className="mt-1 max-w-[22rem] truncate text-xs text-muted-foreground"
                    title={job.reason}
                  >
                    {job.reason}
                  </div>
                )}
              </TableCell>
              <TableCell className="hidden sm:table-cell">
                {job.priority ? (
                  <Badge variant="outline">{job.priority}</Badge>
                ) : (
                  "-"
                )}
              </TableCell>
              <TableCell className="hidden md:table-cell tabular-nums">
                {job.stepCount ?? "-"}
              </TableCell>
              <TableCell className="hidden md:table-cell tabular-nums">
                {formatDuration(job.durationMs)}
              </TableCell>
              <TableCell className="text-right text-muted-foreground">
                {relativeTime(
                  job.finishedAt || job.startedAt || job.queuedAt,
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
