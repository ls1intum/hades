// A tiny pub/sub so the API layer can signal "session is no longer valid"
// (a 401 on any authenticated request) and the auth context can react by
// clearing state and redirecting to the login page.

type Listener = () => void;

const listeners = new Set<Listener>();

/** onUnauthorized subscribes to 401 signals; returns an unsubscribe function. */
export function onUnauthorized(fn: Listener): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

/** emitUnauthorized notifies all subscribers that the session is invalid. */
export function emitUnauthorized(): void {
  listeners.forEach((fn) => fn());
}
