import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { MetadataList } from "./metadata-list";
import { REDACTION_MASK } from "@/lib/types";

describe("MetadataList", () => {
  it("shows visible values and marks redacted ones", () => {
    render(
      <MetadataList
        metadata={{
          REPO_URL: "https://github.com/org/repo.git",
          GIT_PASSWORD: REDACTION_MASK,
        }}
      />,
    );
    expect(screen.getByText("https://github.com/org/repo.git")).toBeInTheDocument();
    expect(screen.getByText("GIT_PASSWORD")).toBeInTheDocument();
    // The masked value renders as "redacted", never the secret.
    expect(screen.getByText("redacted")).toBeInTheDocument();
    expect(screen.queryByText(REDACTION_MASK)).not.toBeInTheDocument();
  });

  it("renders an empty state", () => {
    render(<MetadataList metadata={{}} />);
    expect(screen.getByText("No metadata.")).toBeInTheDocument();
  });
});
