# Go Code Review Rules (Effective Go + Go Code Review Comments)

Review priority: goroutine safety > data race > resource leak > error handling > concurrency patterns > type safety > API design > style.

These rules supplement — not replace — general code review. You MUST still check for logic errors, algorithm correctness, boundary conditions, off-by-one errors, and state management bugs. The rules below focus on Go-specific pitfalls.

Only flag code in the diff. Do not flag inside generated code (protobuf, mockgen), CGo wrappers, or performance-critical code with justifying comments.

## CRITICAL

- **Goroutine leak** → goroutine blocked forever on channel/mutex/IO with no cancellation path. Every long-lived goroutine must select on `<-ctx.Done()` or have a guaranteed channel close. OK only for intentionally immortal goroutines (main loop, daemon). [Effective Go: Concurrency]
- **Data race** → shared mutable state accessed from multiple goroutines without sync.Mutex, sync.RWMutex, atomic, or channel. Includes closure capturing loop variable and mutating shared map/slice. Use `go test -race`. [Go Race Detector]
- **Nil map write** → writing to a nil map panics at runtime. Always `make(map[K]V)` or use literal `map[K]V{}` before write. Reading from nil map is safe (returns zero value). [Effective Go: Maps]
- **Defer in loop** → deferred calls execute at function return, not iteration end. In a loop opening files/connections, resources accumulate until function exits. Extract loop body to separate function, or close explicitly within each iteration. [Effective Go: Defer, CodeReviewComments]
- **sync.WaitGroup.Add inside goroutine** → race between Add and Wait. Always call `wg.Add(n)` before `go func()`. [SA2000]
- **Channel deadlock** → unbuffered channel send/receive with no matching counterpart, or all goroutines blocked. Common: sending to channel in same goroutine that reads it; forgetting to close channel that range-reads. [Go101: Concurrency Mistakes]
- **unsafe.Pointer misuse** → converting to uintptr and back in separate statements allows GC to move the object. The conversion `uintptr(unsafe.Pointer(p)) + offset` must be a single expression. Only the six patterns in the unsafe package doc are valid. [unsafe package docs]
- **Nil pointer from type assertion** → `v := i.(T)` panics if i is nil or wrong type. Use comma-ok form `v, ok := i.(T)` unless panic is intentional and documented. [Effective Go: Interface conversions]

## HIGH

- **Ignored error** → `f()` discarding returned error without explicit assignment or comment. Every error must be checked, wrapped, or explicitly discarded (e.g., `_ = f()` or `_, _ = f()` for multi-return) with a justifying comment. [CodeReviewComments: Handle Errors]
- **Error after err != nil** → using the result value when err != nil. When a function returns `(val, err)`, val may be invalid or zero-valued when err != nil unless the function documents valid partial results. [CodeReviewComments]
- **%v instead of %w** → `fmt.Errorf("...: %v", err)` flattens the error chain; `errors.Is`/`errors.As` will fail. Use `%w` to wrap. Use `%v` only when intentionally hiding the underlying error at an API boundary. [Go Blog: Errors]
- **context.Background() in request path** → loses deadline/cancellation from caller. Propagate the incoming `ctx` from HTTP handler, gRPC call, etc. OK only in main(), init(), or top-level test setup. [CodeReviewComments: Contexts]
- **Context not propagated** → calling a context-aware function with `context.TODO()` or `context.Background()` when a parent context is available. Pass the parent context. [CodeReviewComments: Contexts]
- **Slice append aliasing** → `b := a[:2]; b = append(b, x)` may overwrite `a[2]` if cap allows. When sharing underlying array, use full slice expression `a[:2:2]` to limit capacity, or copy. [Go Blog: Slices]
- **Range variable capture in closure (Go < 1.22)** → `for _, v := range items { go func() { use(v) }() }` captures the loop variable by reference. Fixed in Go 1.22+ (per-iteration scoping). Flag only if go.mod shows Go < 1.22. [Go Wiki: CommonMistakes]
- **Copy of sync type** → copying sync.Mutex, sync.WaitGroup, sync.Cond, or sync.Pool (including via struct assignment or function arg) breaks internal state. Pass by pointer. [go vet copylocks]
- **Mutex unlock not deferred** → manual `mu.Unlock()` skipped on early return or panic. Use `defer mu.Unlock()` immediately after Lock(). OK only when unlock must happen mid-function for performance with justifying comment. [Effective Go: Defer]
- **RWMutex write under RLock** → calling a method that modifies state while holding RLock causes data race. Upgrade to full Lock(). [sync.RWMutex docs]
- **Unclosed resource** → `resp.Body`, `os.File`, `sql.Rows`, `net.Conn` opened but not closed. Use `defer r.Close()` immediately after error check. [CodeReviewComments: Handle Errors]
- **init() with side effects** → init() that opens connections, starts goroutines, or registers global state makes testing and dependency injection difficult. Prefer explicit initialization. OK for simple register-style init (e.g., registering codec, driver). [Go Wiki: CodeReviewComments]

## MEDIUM (suggest, don't block)

- **Stuttering names** → `http.HTTPServer`, `user.UserService`. Package name is part of the qualified name; don't repeat it. [Effective Go: Names]
- **errors.Is/errors.As vs ==** → `err == ErrNotFound` breaks on wrapped errors. Use `errors.Is(err, ErrNotFound)` for sentinel errors, `errors.As(err, &target)` for typed errors. Direct `==` OK only for unwrapped package-internal sentinels. [Go Blog: Errors]
- **Oversized interface** → interface with 5+ methods is hard to implement and mock. Prefer small interfaces (1-2 methods) defined by the consumer. [Go Proverbs, CodeReviewComments: Interfaces]
- **Return interface** → function returning interface hides concrete capabilities. Return concrete type, accept interface. OK when multiple implementations exist (e.g., factory). [CodeReviewComments: Interfaces]
- **Zero-value not usable** → struct requires explicit initialization to be safe. Prefer designs where zero value is valid (e.g., sync.Mutex, bytes.Buffer). [Effective Go: Allocation]
- **Excessive pointer fields** → struct full of `*string`, `*int` when zero values would suffice. Pointer fields require nil checks everywhere. Use zero values or functional options instead. [CodeReviewComments]
- **Table-driven test without t.Run** → parallel tests share test case variable; hard to identify failures. Use `t.Run(tc.name, func(t *testing.T) { ... })`. [Go Wiki: TableDrivenTests]
- **Test helper without t.Helper()** → failure reported at helper's line, not caller's. Add `t.Helper()` as first line in test helper functions. [testing.T.Helper docs]
- **Package-level error string format** → error strings should not be capitalized or end with punctuation, as they are often composed with other messages. [CodeReviewComments: Error Strings]
- **Naked goroutine in library** → `go func()` without lifecycle control (context, done channel) — caller cannot stop or wait for it. Provide shutdown mechanism. OK in main package or CLI tools. [Go Proverbs]

## Decision Trees

### Error handling
```text
Function returns error?
  → error ignored entirely?
      → Flag: check or explicitly discard with _ and comment
  → error checked?
      → result used after err != nil?
          → Flag: result may be invalid when err != nil (check API contract)
      → error wrapped with fmt.Errorf?
          → uses %v?
              → Flag: use %w to preserve error chain (unless hiding intentionally)
          → uses %w?
              → OK
      → error compared?
          → uses == ?
              → unwrapped package-internal sentinel?
                  → OK
              → exported or potentially wrapped?
                  → Flag: use errors.Is for sentinel, errors.As for typed
          → uses errors.Is/errors.As?
              → OK
```

### Goroutine lifecycle
```text
New goroutine spawned?
  → has context or done channel?
      → selects on ctx.Done() / done?
          → OK
      → never checks cancellation?
          → Flag: goroutine may leak
  → no cancellation mechanism?
      → short-lived, bounded work?
          → OK (but suggest context)
      → long-lived or unbounded?
          → Flag: goroutine leak risk

  → uses sync.WaitGroup?
      → Add() called before go func()?
          → OK
      → Add() called inside goroutine?
          → Flag: race between Add and Wait
```

### Interface design
```text
New interface defined?
  → has 5+ methods?
      → Flag: split into smaller interfaces
  → has 1-2 methods?
      → OK

  → defined by producer (same package as implementation)?
      → only one implementation exists?
          → Flag: premature abstraction — use concrete type
      → multiple implementations?
          → OK

  → defined by consumer?
      → OK — idiomatic Go
```
