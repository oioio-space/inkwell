// SPDX-License-Identifier: MPL-2.0
/** A complete case from the shared JSON corpus; extensions are retained. */
export interface Vector {
  id: string;
  decoder?: string;
  status: string;
  enforce?: string;
  verdict: string;
  input_hex?: string;
  inputs_hex?: string[];
  expect?: Record<string, unknown>;
  [field: string]: unknown;
}
/** A case verdict, including informational failures from future migration steps. */
export interface Result {
  vector: Vector;
  required: boolean;
  passed: boolean;
  error?: unknown;
}
/** Execute every supplied case. A check succeeds only when verdict and expectations agree. */
export async function runConformance(
  vectors: Vector[],
  check: (vector: Vector) => void | Promise<void>,
  step = 1,
): Promise<Result[]> {
  const results: Result[] = [];
  for (const vector of vectors) {
    const required = vector.enforce
      ? Number(vector.enforce.replace("step-", "")) <= step
      : vector.status === "normative";
    try {
      await check(vector);
      results.push({ vector, required, passed: true });
    } catch (error) {
      results.push({ vector, required, passed: false, error });
    }
  }
  return results;
}
