import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { patchJob } from "./useStream";
import type { JobSummary } from "@/lib/types";

function job(id: string, status: JobSummary["status"], name = id): JobSummary {
  return { id, name, status };
}

describe("patchJob (SSE cache patching)", () => {
  it("adds a new job to the unfiltered list", () => {
    const qc = new QueryClient();
    qc.setQueryData(["jobs", ""], [job("a", "Queued")]);
    patchJob(qc, job("b", "Queued"));
    const list = qc.getQueryData<JobSummary[]>(["jobs", ""])!;
    expect(list.map((j) => j.id)).toEqual(["b", "a"]);
  });

  it("respects a status filter: keeps matching, drops non-matching", () => {
    const qc = new QueryClient();
    qc.setQueryData(["jobs", "Running"], [job("a", "Running")]);
    // 'a' transitions to Succeeded -> must leave the Running list.
    patchJob(qc, job("a", "Succeeded"));
    expect(qc.getQueryData<JobSummary[]>(["jobs", "Running"])).toEqual([]);

    // A new Running job is added to the Running list.
    patchJob(qc, job("c", "Running"));
    expect(qc.getQueryData<JobSummary[]>(["jobs", "Running"])!.map((j) => j.id)).toEqual(["c"]);
  });

  it("does not add a non-matching new job to a filtered list", () => {
    const qc = new QueryClient();
    qc.setQueryData(["jobs", "Failed"], [] as JobSummary[]);
    patchJob(qc, job("x", "Running"));
    expect(qc.getQueryData<JobSummary[]>(["jobs", "Failed"])).toEqual([]);
  });

  it("caps list growth", () => {
    const qc = new QueryClient();
    const many = Array.from({ length: 500 }, (_, i) => job(`j${i}`, "Queued"));
    qc.setQueryData(["jobs", ""], many);
    patchJob(qc, job("new", "Queued"));
    const list = qc.getQueryData<JobSummary[]>(["jobs", ""])!;
    expect(list.length).toBe(500);
    expect(list[0].id).toBe("new");
  });

  it("merges into an open detail view", () => {
    const qc = new QueryClient();
    qc.setQueryData(["job", "a"], { id: "a", status: "Running", steps: [] });
    patchJob(qc, job("a", "Succeeded"));
    expect(qc.getQueryData<{ status: string }>(["job", "a"])!.status).toBe("Succeeded");
  });
});
