import { execSync } from "node:child_process";

/** Removes the NATS container started by serve.sh. */
export default function globalTeardown() {
  try {
    execSync("docker rm -f hades-e2e-nats", { stdio: "ignore" });
  } catch {
    // Container may already be gone (e.g. reuseExistingServer); ignore.
  }
}
