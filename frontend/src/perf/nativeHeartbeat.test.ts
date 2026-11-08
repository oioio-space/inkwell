import { describe, expect, it } from "vitest";
import { ContinuousHeartbeat } from "./nativeHeartbeat";

const gaps = (points: readonly number[]) =>
  points.slice(1).map((time, i) => time - (points[i] ?? time));
describe("continuous native heartbeat evidence", () => {
  it("includes a >50ms idle gap between completed sample and next arm", () => {
    const heartbeat = new ContinuousHeartbeat();
    heartbeat.push(0);
    heartbeat.push(16);
    heartbeat.complete();
    // No trial is active while these callbacks arrive.
    heartbeat.push(120);
    heartbeat.push(136);
    expect(heartbeat.snapshot()).toEqual([16, 120, 136]);
    expect(Math.max(...gaps(heartbeat.snapshot()))).toBe(104);
  });
  it("does not carry a completed control's stall into the next normal trial", () => {
    const heartbeat = new ContinuousHeartbeat();
    heartbeat.push(0);
    heartbeat.push(16);
    heartbeat.push(112);
    heartbeat.push(128);
    expect(Math.max(...gaps(heartbeat.snapshot()))).toBe(96);
    heartbeat.complete();
    heartbeat.push(144);
    heartbeat.push(160);
    expect(heartbeat.snapshot()).toEqual([128, 144, 160]);
    expect(Math.max(...gaps(heartbeat.snapshot()))).toBe(16);
  });
  it("bounds storage without silently discarding any gap evidence", () => {
    const heartbeat = new ContinuousHeartbeat(2);
    expect(heartbeat.push(0)).toBe(true);
    expect(heartbeat.push(100)).toBe(true);
    expect(heartbeat.push(116)).toBe(false);
    expect(heartbeat.snapshot()).toEqual([0, 100]);
    heartbeat.complete();
    expect(heartbeat.push(116)).toBe(true);
  });
});
