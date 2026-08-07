import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { StatusBadge } from "@/components/status-badge";
import { MetadataList } from "@/components/metadata-list";
import { LogsViewer } from "@/components/logs-viewer";
import { formatDuration, formatTime } from "@/lib/utils";
import type { JobStatus } from "@/lib/types";

export function JobDetailPage() {
  const { id = "" } = useParams();
  const job = useQuery({
    queryKey: ["job", id],
    queryFn: () => api.job(id),
    refetchInterval: 10000,
  });

  return (
    <div className="space-y-6">
      <Button variant="ghost" size="sm" asChild>
        <Link to="/jobs">
          <ArrowLeft /> Back to jobs
        </Link>
      </Button>

      {job.isLoading ? (
        <Skeleton className="h-24 w-full" />
      ) : job.isError || !job.data ? (
        <div className="rounded-lg border border-dashed p-10 text-center text-sm text-muted-foreground">
          Job not found.
        </div>
      ) : (
        <>
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="space-y-1">
              <div className="flex items-center gap-3">
                <h1 className="text-2xl font-semibold tracking-tight">
                  {job.data.name || "(unnamed job)"}
                </h1>
                <StatusBadge status={job.data.status} />
              </div>
              <p className="font-mono text-xs text-muted-foreground">{job.data.id}</p>
            </div>
            <div className="flex flex-wrap gap-2 text-sm">
              {job.data.priority && (
                <Badge variant="outline">priority: {job.data.priority}</Badge>
              )}
              <Badge variant="outline">
                duration: {formatDuration(job.data.durationMs)}
              </Badge>
            </div>
          </div>

          <div className="grid gap-3 text-sm sm:grid-cols-3">
            <TimeField label="Queued" value={formatTime(job.data.queuedAt)} />
            <TimeField label="Started" value={formatTime(job.data.startedAt)} />
            <TimeField label="Finished" value={formatTime(job.data.finishedAt)} />
          </div>

          <Tabs defaultValue="steps">
            <TabsList>
              <TabsTrigger value="steps">
                Steps ({job.data.steps.length})
              </TabsTrigger>
              <TabsTrigger value="metadata">Metadata</TabsTrigger>
              <TabsTrigger value="logs">Logs</TabsTrigger>
            </TabsList>

            <TabsContent value="steps" className="space-y-3">
              {!job.data.payloadAvailable && (
                <p className="text-sm text-muted-foreground">
                  The full job definition has aged out of storage.
                </p>
              )}
              {job.data.steps.map((step) => (
                <Card key={step.id}>
                  <CardHeader className="pb-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <CardTitle className="text-base">
                        {step.id}. {step.name || step.image}
                      </CardTitle>
                      <Badge variant="secondary">{step.image}</Badge>
                      {step.continueOnError && (
                        <Badge variant="warning">continueOnError</Badge>
                      )}
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    {step.script && (
                      <pre className="max-h-64 overflow-auto rounded-md bg-muted p-3 font-mono text-xs">
                        {step.script}
                      </pre>
                    )}
                    <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
                      {step.cpuLimit > 0 && (
                        <Badge variant="outline">cpu: {step.cpuLimit}m</Badge>
                      )}
                      {step.memoryLimit && (
                        <Badge variant="outline">mem: {step.memoryLimit}</Badge>
                      )}
                    </div>
                    {Object.keys(step.metadata ?? {}).length > 0 && (
                      <MetadataList metadata={step.metadata} />
                    )}
                  </CardContent>
                </Card>
              ))}
            </TabsContent>

            <TabsContent value="metadata">
              <MetadataList metadata={job.data.metadata} />
            </TabsContent>

            <TabsContent value="logs">
              <LogsViewer jobId={job.data.id} status={job.data.status as JobStatus} />
            </TabsContent>
          </Tabs>
        </>
      )}
    </div>
  );
}

function TimeField({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border px-3 py-2">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="tabular-nums">{value}</div>
    </div>
  );
}
