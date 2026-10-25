/**
 * Runs `task` when the browser is idle (a timeout where `requestIdleCallback` is missing, as
 * in WebKit), so counting words never competes with typing. Returns a cancel function.
 */
export function whenIdle(task: () => void, timeout = 400): () => void {
  if (typeof requestIdleCallback === "function") {
    const id = requestIdleCallback(task, { timeout });
    return () => cancelIdle(id);
  }
  const id = setTimeout(task, 50);
  return () => clearTimeout(id);
}

function cancelIdle(id: number): void {
  if (typeof cancelIdleCallback === "function") cancelIdleCallback(id);
}
