import { Badge } from "@/components/ui/badge";
import type { JobStatus } from "@/lib/types";

const variantByStatus: Record<
  JobStatus,
  "default" | "secondary" | "success" | "warning" | "info" | "destructive"
> = {
  Queued: "secondary",
  Running: "info",
  Succeeded: "success",
  Failed: "destructive",
  Stopped: "warning",
  Unknown: "secondary",
};

export function StatusBadge({ status }: { status: JobStatus }) {
  const variant = variantByStatus[status] ?? "secondary";
  return (
    <Badge variant={variant} aria-label={`status ${status}`}>
      {status}
    </Badge>
  );
}
