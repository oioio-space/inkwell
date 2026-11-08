/** Raw callback arrivals since the previous sample's final observed frame. */
export class ContinuousHeartbeat {
  #points: number[] = [];
  constructor(readonly capacity = 4096) {}
  push(time: number): boolean {
    const last = this.#points.at(-1);
    if (
      !Number.isFinite(time) ||
      time < 0 ||
      (last !== undefined && time < last) ||
      this.#points.length >= this.capacity
    )
      return false;
    this.#points.push(time);
    return true;
  }
  snapshot(): number[] {
    return [...this.#points];
  }
  /** Retain the post-control boundary, without carrying the completed trial's own stall. */
  complete(): void {
    const last = this.#points.at(-1);
    this.#points = last === undefined ? [] : [last];
  }
}
