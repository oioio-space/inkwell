---
name: use-modern-go
description: Use BEFORE writing, editing, or reviewing ANY Go code in this project — and whenever the user asks for modern Go guidelines. Applies modern Go syntax/idioms for the project's detected Go version (slices, maps, cmp, min/max, range-over-int, t.Context, omitzero, wg.Go, new(val), errors.AsType, …) and avoids outdated patterns.
---

# Modern Go Guidelines

## Detected Go Version

!`grep -rh "^go " --include="go.mod" . 2>/dev/null | cut -d' ' -f2 | sort | uniq -c | sort -nr | head -1 | xargs | cut -d' ' -f2 | grep . || echo unknown`

## How to Use This Skill

DO NOT search for go.mod files or try to detect the version yourself. Use ONLY the version shown above.

**If version detected (not "unknown"):**
- Say: "This project is using Go X.XX, so I’ll stick to modern Go best practices and freely use language features up to and including this version. If you’d prefer a different target version, just let me know."
- Do NOT list features, do NOT ask for confirmation

**If version is "unknown":**
- Say: "Could not detect Go version in this repository"
- Use AskUserQuestion: "Which Go version should I target?" → [1.23] / [1.24] / [1.25] / [1.26]

**When writing Go code**, use ALL features from this document up to the target version:
- Prefer modern built-ins and packages (`slices`, `maps`, `cmp`) over legacy patterns
- Never use features from newer Go versions than the target
- Never use outdated patterns when a modern alternative is available

---

## Features by Go Version

### Go 1.0+

- `time.Since`: `time.Since(start)` instead of `time.Now().Sub(start)`

### Go 1.8+

- `time.Until`: `time.Until(deadline)` instead of `deadline.Sub(time.Now())`

### Go 1.13+

- `errors.Is`: `errors.Is(err, target)` instead of `err == target` (works with wrapped errors)

### Go 1.18+

- `any`: Use `any` instead of `interface{}`
- `bytes.Cut`: `before, after, found := bytes.Cut(b, sep)` instead of Index+slice
- `strings.Cut`: `before, after, found := strings.Cut(s, sep)`

### Go 1.19+

- `fmt.Appendf`: `buf = fmt.Appendf(buf, "x=%d", x)` instead of `[]byte(fmt.Sprintf(...))`
- `atomic.Bool`/`atomic.Int64`/`atomic.Pointer[T]`: Type-safe atomics instead of `atomic.StoreInt32`

```go
var flag atomic.Bool
flag.Store(true)
if flag.Load() { ... }

var ptr atomic.Pointer[Config]
ptr.Store(cfg)
```

### Go 1.20+

- `strings.Clone`: `strings.Clone(s)` to copy string without sharing memory
- `bytes.Clone`: `bytes.Clone(b)` to copy byte slice
- `strings.CutPrefix/CutSuffix`: `if rest, ok := strings.CutPrefix(s, "pre:"); ok { ... }`
- `errors.Join`: `errors.Join(err1, err2)` to combine multiple errors
- `context.WithCancelCause`: `ctx, cancel := context.WithCancelCause(parent)` then `cancel(err)`
- `context.Cause`: `context.Cause(ctx)` to get the error that caused cancellation

### Go 1.21+

**Built-ins:**
- `min`/`max`: `max(a, b)` instead of if/else comparisons
- `clear`: `clear(m)` to delete all map entries, `clear(s)` to zero slice elements

**slices package:**
- `slices.Contains`: `slices.Contains(items, x)` instead of manual loops
- `slices.Index`: `slices.Index(items, x)` returns index (-1 if not found)
- `slices.IndexFunc`: `slices.IndexFunc(items, func(item T) bool { return item.ID == id })`
- `slices.SortFunc`: `slices.SortFunc(items, func(a, b T) int { return cmp.Compare(a.X, b.X) })`
- `slices.Sort`: `slices.Sort(items)` for ordered types
- `slices.Max`/`slices.Min`: `slices.Max(items)` instead of manual loop
- `slices.Reverse`: `slices.Reverse(items)` instead of manual swap loop
- `slices.Compact`: `slices.Compact(items)` removes consecutive duplicates in-place
- `slices.Clip`: `slices.Clip(s)` removes unused capacity
- `slices.Clone`: `slices.Clone(s)` creates a copy

**maps package:**
- `maps.Clone`: `maps.Clone(m)` instead of manual map iteration
- `maps.Copy`: `maps.Copy(dst, src)` copies entries from src to dst
- `maps.DeleteFunc`: `maps.DeleteFunc(m, func(k K, v V) bool { return condition })`

**sync package:**
- `sync.OnceFunc`: `f := sync.OnceFunc(func() { ... })` instead of `sync.Once` + wrapper
- `sync.OnceValue`: `getter := sync.OnceValue(func() T { return computeValue() })`

**context package:**
- `context.AfterFunc`: `stop := context.AfterFunc(ctx, cleanup)` runs cleanup on cancellation
- `context.WithTimeoutCause`: `ctx, cancel := context.WithTimeoutCause(parent, d, err)`
- `context.WithDeadlineCause`: Similar with deadline instead of duration

### Go 1.22+

**Loops:**
- `for i := range n`: `for i := range len(items)` instead of `for i := 0; i < len(items); i++`
- Loop variables are now safe to capture in goroutines (each iteration has its own copy)

**cmp package:**
- `cmp.Or`: `cmp.Or(flag, env, config, "default")` returns first non-zero value

```go
// Instead of:
name := os.Getenv("NAME")
if name == "" {
    name = "default"
}
// Use:
name := cmp.Or(os.Getenv("NAME"), "default")
```

**reflect package:**
- `reflect.TypeFor`: `reflect.TypeFor[T]()` instead of `reflect.TypeOf((*T)(nil)).Elem()`

**net/http:**
- Enhanced `http.ServeMux` patterns: `mux.HandleFunc("GET /api/{id}", handler)` with method and path params
- `r.PathValue("id")` to get path parameters

### Go 1.23+

- `maps.Keys(m)` / `maps.Values(m)` return iterators
- `slices.Collect(iter)` not manual loop to build slice from iterator
- `slices.Sorted(iter)` to collect and sort in one step

```go
keys := slices.Collect(maps.Keys(m))       // not: for k := range m { keys = append(keys, k) }
sortedKeys := slices.Sorted(maps.Keys(m))  // collect + sort
for k := range maps.Keys(m) { process(k) } // iterate directly
```

**time package**

- `time.Tick`: Use `time.Tick` freely — as of Go 1.23, the garbage collector can recover unreferenced tickers, even if they haven't been stopped. The Stop method is no longer necessary to help the garbage collector. There is no longer any reason to prefer NewTicker when Tick will do.

### Go 1.24+

- `t.Context()` not `context.WithCancel(context.Background())` in tests.
  ALWAYS use t.Context() when a test function needs a context.

Before:
```go
func TestFoo(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    result := doSomething(ctx)
}
```
After:
```go
func TestFoo(t *testing.T) {
    ctx := t.Context()
    result := doSomething(ctx)
}
```

- `omitzero` not `omitempty` in JSON struct tags.
  ALWAYS use omitzero for time.Duration, time.Time, structs, slices, maps.

Before:
```go
type Config struct {
    Timeout time.Duration `json:"timeout,omitempty"` // doesn't work for Duration!
}
```
After:
```go
type Config struct {
    Timeout time.Duration `json:"timeout,omitzero"`
}
```

- `b.Loop()` not `for i := 0; i < b.N; i++` in benchmarks.
  ALWAYS use b.Loop() for the main loop in benchmark functions.

Before:
```go
func BenchmarkFoo(b *testing.B) {
    for i := 0; i < b.N; i++ {
        doWork()
    }
}
```
After:
```go
func BenchmarkFoo(b *testing.B) {
    for b.Loop() {
        doWork()
    }
}
```

- `strings.SplitSeq` not `strings.Split` when iterating.
  ALWAYS use SplitSeq/FieldsSeq when iterating over split results in a for-range loop.

Before:
```go
for _, part := range strings.Split(s, ",") {
    process(part)
}
```
After:
```go
for part := range strings.SplitSeq(s, ",") {
    process(part)
}
```
Also: `strings.FieldsSeq`, `bytes.SplitSeq`, `bytes.FieldsSeq`.

### Go 1.25+

- `wg.Go(fn)` not `wg.Add(1)` + `go func() { defer wg.Done(); ... }()`.
  ALWAYS use wg.Go() when spawning goroutines with sync.WaitGroup.

Before:
```go
var wg sync.WaitGroup
for _, item := range items {
    wg.Add(1)
    go func() {
        defer wg.Done()
        process(item)
    }()
}
wg.Wait()
```
After:
```go
var wg sync.WaitGroup
for _, item := range items {
    wg.Go(func() {
        process(item)
    })
}
wg.Wait()
```

### Go 1.26+

- `new(val)` not `x := val; &x` — returns pointer to any value.
  Go 1.26 extends new() to accept expressions, not just types.
  Type is inferred: new(0) → *int, new("s") → *string, new(T{}) → *T.
  DO NOT use `x := val; &x` pattern — always use new(val) directly.
  DO NOT use redundant casts like new(int(0)) — just write new(0).
  Common use case: struct fields with pointer types.

Before:
```go
timeout := 30
debug := true
cfg := Config{
    Timeout: &timeout,
    Debug:   &debug,
}
```
After:
```go
cfg := Config{
    Timeout: new(30),   // *int
    Debug:   new(true), // *bool
}
```

- `errors.AsType[T](err)` not `errors.As(err, &target)`.
  ALWAYS use errors.AsType when checking if error matches a specific type.

Before:
```go
var pathErr *os.PathError
if errors.As(err, &pathErr) {
    handle(pathErr)
}
```
After:
```go
if pathErr, ok := errors.AsType[*os.PathError](err); ok {
    handle(pathErr)
}
```

### Go 1.27+

Source: https://go.dev/doc/go1.27. `go fix ./...` applies the modernizers, and the golangci-lint
`modernize` linter (`.golangci.yml`) refuses outdated idioms: run `go fix` before linting.

- Struct literal keys may name a promoted field through an embedded struct (modernizer `embedlit`).

Before:
```go
p := Project{Meta: Meta{ID: id, Title: title}, Root: root}
```
After:
```go
p := Project{ID: id, Title: title, Root: root}
```

- Generic methods: a method may declare its own type parameters (not on interface methods, and a
  generic method cannot implement an interface method). Prefer one generic method over N copies.

```go
func (c *Cache) Get[T any](key string) (T, bool) { ... }
```

- `strings.CutLast` / `bytes.CutLast`: cut around the LAST separator instead of `LastIndex` + slicing.

```go
dir, base, ok := strings.CutLast(path, "/")
```

- `slices.Backward` instead of a reverse index loop (modernizer `slicesbackward`).

```go
for i, x := range slices.Backward(items) { ... } // not: for i := len(items)-1; i >= 0; i--
```

- `atomic.Int64`/`atomic.Bool`/`atomic.Pointer[T]` instead of `atomic.AddInt64(&x, 1)` on plain
  fields (modernizer `atomictypes`); `unsafe.Slice`/`unsafe.String`/`unsafe.StringData` instead of
  `reflect.SliceHeader` tricks (modernizer `unsafefuncs`).
- `encoding/json/v2` (`json.Marshal(v, opts...)`, `UnmarshalRead`) for NEW JSON code: it rejects
  invalid UTF-8 and duplicate object names by default. `encoding/json` v1 is now backed by v2;
  don't migrate existing v1 code without a reason (error texts differ).
- `testing/synctest.Sleep(d)` instead of `time.Sleep(d)` followed by `synctest.Wait()`.
- `net/url.URL.Clone` / `url.Values.Clone` instead of copying by hand; `math/rand/v2`:
  `r.N(10*time.Second)` (generic method).
- `httptest.NewTestServer`: an HTTP server on an in-memory network, for `testing/synctest`.
- `pprof.Lookup("goroutineleak")` is GA (no GOEXPERIMENT): see skill `goroutine-leak-check`.
- `go vet`: `fmt.Errorf("…: %w", p)` with `p` of type `*E` where `E` implements `error` is now
  reported: wrap the value whose type implements `error`.
- `go test` runs `stdversion`: no standard-library symbol newer than the file's `go` version.
- Do NOT use `fmt.Appendf` just to replace `[]byte(fmt.Sprintf(...))`: the `fmtappendf` modernizer
  was removed in 1.27 for stylistic reasons; use it only when appending to an existing buffer.
