import "@testing-library/jest-dom/vitest";

// jsdom lacks matchMedia (used by the theme provider) and EventSource (SSE).
if (!window.matchMedia) {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}

class MockEventSource {
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
}
if (!("EventSource" in globalThis)) {
  (globalThis as unknown as { EventSource: unknown }).EventSource =
    MockEventSource;
}
