import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { JobsTable } from "./jobs-table";
import { renderWithProviders } from "@/test/utils";
import type { JobSummary } from "@/lib/types";

const jobs: JobSummary[] = [
  {
    id: "abcdef12-0000-0000-0000-000000000000",
    name: "build-app",
    status: "Running",
    priority: "high",
    stepCount: 3,
    durationMs: 4200,
    startedAt: new Date().toISOString(),
  },
];

describe("JobsTable", () => {
  it("renders job rows", () => {
    renderWithProviders(<JobsTable jobs={jobs} />);
    expect(screen.getByText("build-app")).toBeInTheDocument();
    expect(screen.getByText("Running")).toBeInTheDocument();
    expect(screen.getByText("high")).toBeInTheDocument();
    // Short id prefix is shown.
    expect(screen.getByText("abcdef12")).toBeInTheDocument();
  });

  it("renders an empty state", () => {
    renderWithProviders(<JobsTable jobs={[]} />);
    expect(screen.getByText(/No jobs/i)).toBeInTheDocument();
  });

  it("renders a loading skeleton", () => {
    const { container } = renderWithProviders(<JobsTable loading />);
    expect(container.querySelector(".animate-pulse")).toBeTruthy();
  });

  it("renders an error state distinct from empty", () => {
    renderWithProviders(<JobsTable jobs={[]} error />);
    expect(screen.getByRole("alert")).toHaveTextContent(/failed to load/i);
  });

  it("job rows are keyboard-accessible", () => {
    renderWithProviders(<JobsTable jobs={jobs} />);
    const row = screen.getByRole("link", { name: /open job build-app/i });
    expect(row).toHaveAttribute("tabindex", "0");
  });
});
