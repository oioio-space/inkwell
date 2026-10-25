// Vite empties dist/ on build; restore the placeholder Go's //go:embed needs so the
// module compiles even before the first frontend build (see inkwell.go).
import { writeFileSync } from "node:fs";

writeFileSync(new URL("../dist/.gitkeep", import.meta.url), "");
