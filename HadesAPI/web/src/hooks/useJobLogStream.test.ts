import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { appendLogGroup } from "./useJobLogStream";
import type { LogEntry, LogGroup, LogsResponse } from "@/lib/types";

function entry(ts: string, message: string, stream = "stdout"): LogEntry {
  return { timestamp: ts, message, output_stream: stream };
}

function group(container: string, logs: LogEntry[]): LogGroup {
  return { job_id: "job-1", container_id: container, logs };
}

function cached(qc: QueryClient): LogsResponse | undefined {
  return qc.getQueryData<LogsResponse>(["logs", "job-1"]);
}

describe("appendLogGroup (live log SSE cache merge)", () => {
  it("creates a container group on first sight", () => {
    const qc = new QueryClient();
    appendLogGroup(qc, "job-1", group("step-1", [entry("t1", "a")]));
    expect(cached(qc)!.logs).toEqual([group("step-1", [entry("t1", "a")])]);
  });

  it("appends incremental batches into the same container in order", () => {
    const qc = new QueryClient();
    appendLogGroup(qc, "job-1", group("step-1", [entry("t1", "a")]));
    appendLogGroup(qc, "job-1", group("step-1", [entry("t2", "b")]));
    expect(cached(qc)!.logs[0].logs.map((e) => e.message)).toEqual(["a", "b"]);
  });

  it("keeps containers in first-seen order, preserving the step index", () => {
    const qc = new QueryClient();
    appendLogGroup(qc, "job-1", group("step-1", [])); // zero-output slot
    appendLogGroup(qc, "job-1", group("step-2", [entry("t1", "build")]));
    const logs = cached(qc)!.logs;
    expect(logs.map((g) => g.container_id)).toEqual(["step-1", "step-2"]);
    expect(logs[0].logs).toEqual([]);
  });

  it("dedupes entries by timestamp+stream+message (replay safe)", () => {
    const qc = new QueryClient();
    appendLogGroup(qc, "job-1", group("step-1", [entry("t1", "a"), entry("t2", "b")]));
    // A reconnect replays the same batch plus a new line.
    appendLogGroup(
      qc,
      "job-1",
      group("step-1", [entry("t1", "a"), entry("t2", "b"), entry("t3", "c")]),
    );
    expect(cached(qc)!.logs[0].logs.map((e) => e.message)).toEqual(["a", "b", "c"]);
  });

  it("treats same text on different streams as distinct", () => {
    const qc = new QueryClient();
    appendLogGroup(qc, "job-1", group("step-1", [entry("t1", "x", "stdout")]));
    appendLogGroup(qc, "job-1", group("step-1", [entry("t1", "x", "stderr")]));
    expect(cached(qc)!.logs[0].logs).toHaveLength(2);
  });
});
