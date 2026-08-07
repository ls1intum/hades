// Types mirror the HadesAPI dashboard JSON contract (HadesAPI/dashboard).

export type JobStatus =
  | "Queued"
  | "Running"
  | "Succeeded"
  | "Failed"
  | "Stopped"
  | "Unknown";

export interface JobSummary {
  id: string;
  name?: string;
  priority?: string;
  status: JobStatus;
  stepCount?: number;
  queuedAt?: string;
  startedAt?: string;
  finishedAt?: string;
  durationMs?: number;
}

export interface StepView {
  id: number;
  name: string;
  image: string;
  script: string;
  continueOnError: boolean;
  metadata: Record<string, string>;
  cpuLimit: number;
  memoryLimit: string;
}

export interface JobDetail extends JobSummary {
  timestamp?: string;
  metadata: Record<string, string>;
  steps: StepView[];
  payloadAvailable: boolean;
}

export interface LogEntry {
  timestamp: string;
  message: string;
  output_stream: string;
}

export interface LogGroup {
  job_id: string;
  container_id: string;
  logs: LogEntry[];
}

export interface LogsResponse {
  logs: LogGroup[];
}

export interface QueueDepth {
  total: number;
  byPriority: Record<string, number>;
  approximate: boolean;
}

export interface Durations {
  avgMs: number;
  p95Ms: number;
  count: number;
}

export interface Metrics {
  statusCounts: Record<string, number>;
  queueDepth: QueueDepth;
  durations: Durations;
  throughputPerMin: number;
  streamClients: number;
  timestamp: string;
}

/** Mask token used by the server for redacted metadata values. */
export const REDACTION_MASK = "••••••";
