import { EyeOff } from "lucide-react";
import { REDACTION_MASK } from "@/lib/types";

/**
 * MetadataList renders a job's metadata. Values the server redacted arrive as
 * the mask token; we render them as a muted "redacted" chip so operators can see
 * which variables exist without exposing secrets.
 */
export function MetadataList({ metadata }: { metadata: Record<string, string> }) {
  const entries = Object.entries(metadata ?? {});
  if (entries.length === 0) {
    return <p className="text-sm text-muted-foreground">No metadata.</p>;
  }
  return (
    <dl className="divide-y rounded-md border text-sm">
      {entries.map(([key, value]) => {
        const redacted = value === REDACTION_MASK;
        return (
          <div key={key} className="flex items-center gap-3 px-3 py-2">
            <dt className="w-1/3 shrink-0 font-mono text-xs text-muted-foreground">
              {key}
            </dt>
            <dd className="min-w-0 flex-1 break-all font-mono text-xs">
              {redacted ? (
                <span className="inline-flex items-center gap-1 text-muted-foreground">
                  <EyeOff className="size-3" />
                  redacted
                </span>
              ) : (
                value
              )}
            </dd>
          </div>
        );
      })}
    </dl>
  );
}
