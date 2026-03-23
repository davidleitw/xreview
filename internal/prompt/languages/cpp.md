# C++ Code Review Rules (ISO C++ Core Guidelines)

Review priority: memory safety > UB > concurrency > exception safety > ownership > types > class design > style.

These rules supplement — not replace — general code review. You MUST still check for logic errors, algorithm correctness, boundary conditions, off-by-one errors, and state management bugs. The rules below focus on C++-specific pitfalls.

Only flag code in the diff. Do not flag inside custom allocators, C interop wrappers, or performance-critical code with justifying comments.

## CRITICAL

- **Naked new/delete** in application code → use make_unique/make_shared. OK only in: RAII class ctor/dtor that manages ownership, custom allocators, placement-new. [R.11, R.3]
- **Mismatched new/delete[]** → always UB, no exceptions. [R.11]
- **Return pointer/ref to local** → dangling. Also watch: returning string_view of local string, iterator of local container. [F.43, F.44]
- **Use-after-move** → moved-from object is in valid but unspecified state (class invariants hold). Safe: assign, destroy, or no-precondition queries (empty(), size(), clear()). Unsafe: any operation with preconditions (front(), back(), operator* on empty container/pointer) — precondition may not hold, causing UB. [ES.56, C.64]
- **RAII gap** → resource acquired (fopen, open, new) but not immediately wrapped in RAII. OK only if pointer is clearly non-owning (not responsible for cleanup, lifetime guaranteed by documented owner). [R.1]
- **memcpy/memset on non-trivially-copyable type** → corrupts vtable, string internals, smart pointers. OK only for types satisfying std::is_trivially_copyable (scalars, C structs, plain arrays of these). When in doubt check `std::is_trivially_copyable_v<T>` — types containing any STL container, string, smart pointer, or optional are NOT trivially copyable. [SL.con.4]
- **Unnamed RAII guard** → `lock_guard{m};` destroys immediately, locks nothing. Must be named. [CP.44, ES.84]
- **Non-virtual dtor in polymorphic base** → UB when deleting derived through base pointer. Must be public virtual or protected non-virtual. OK for final classes, CRTP bases. [C.35, C.127]
- **Throw in destructor** → destructors are implicitly noexcept; throwing calls std::terminate. [C.36, E.16]
- **Data race** → shared mutable state without mutex/atomic. Note: static local *initialization* is thread-safe (C++11 Magic Statics), but subsequent mutation is NOT. [CP.2]
- **volatile for synchronization** → volatile provides no atomicity or ordering in C++. Use std::atomic. volatile is only for hardware registers. [CP.8, CP.200]
- **Array-of-derived via base pointer** → stride mismatch causes UB. `void f(Base* p, int n); Derived d[10]; f(d,10);` is broken. [C.152]
- **strcpy/sprintf/gets** and other unbounded C string functions → buffer overflow. Use std::string, snprintf, or gsl::span. [SL.str.1]
- **Dangling c_str()/data()** → two common patterns:
  - *Mutation after capture*: `const char* s = str.c_str(); str += "x"; use(s);` invalidates s. Pin the string or use immediately.
  - *Temporary*: `const char* s = (a + b).c_str();` or `const char* s = f().c_str();` — temporary string is destroyed at end of full-expression, pointer dangles immediately. Pin the string to a named variable first.

## HIGH

- **Ownership ambiguity** → T* returning from function: is caller supposed to delete? T* = non-owning. Use unique_ptr for ownership transfer, shared_ptr for shared. OK for non-owning observers into containers/members. [R.3, I.11, F.26, F.27]
- **C-style cast** → `(T)x` always flag. `T(x)` flag only when T is scalar/pointer type; `T(x)` where T is class type = construction, OK. [ES.49]
- **const_cast removing const** → UB if original object is const. OK only for C API interop that doesn't modify. [ES.50]
- **reinterpret_cast** → flag in application code. OK in low-level/serialization/hardware code if encapsulated. [Pro.safety]
- **static_cast downcast** → use dynamic_cast for polymorphic types. OK for CRTP or statically-known type. [C.146]
- **Rule of Five violation** → if any of {dtor, copy ctor, copy assign, move ctor, move assign} is user-defined or deleted, define/delete all five. Exception per C.21: polymorphic base with only `virtual ~Base() = default;` is acceptable, but declaring the virtual dtor blocks implicit moves (copies deprecated), so all five should be explicitly `= default` or `= delete`. See also C.67 below. [C.21]
- **Public copy/move on polymorphic class** → enables slicing. Delete or make protected. Provide virtual clone() if copyability is needed. [C.67]
- **Virtual call in ctor/dtor** → dispatches to current class, not derived. Almost never intended. [C.82]
- **Escaping lambda with [&]** → reference capture dangling if lambda is stored, returned, or passed to thread/async. Capture by value for escaping lambdas. [F.53]
- **va_arg** → not type-safe. Use variadic templates. [F.55]
- **Manual lock()/unlock()** → use lock_guard/unique_lock/scoped_lock. Manual unlock skipped by exceptions. [CP.20]
- **Multiple mutex without scoped_lock** → deadlock risk. Use std::scoped_lock(m1, m2). [CP.21]
- **thread::detach()** → unmonitorable, outlives referenced objects. Prefer joining threads. [CP.26]
- **cv.wait() without predicate** → spurious wakeup. Always cv.wait(lock, pred). [CP.42]
- **Coroutine + reference capture/params** → dangle after first suspension. Pass by value.
- **Lock held across co_await** → deadlock. Release before suspension.
- **Catch by value** → slices derived exception. Use `catch(const E&)`. [E.15]
- **throw e instead of throw** → copy-initializes a new exception object from the declared catch type; slices if caught as base (the typical case). Use bare `throw;` to rethrow the original object without copy or type change. [E.15]
- **Move ops not noexcept** → vector falls back to copy on realloc. Mark noexcept. [C.66]
- **using namespace in header** → pollutes all includers. OK in .cpp or local scope. [SF.7]
- **Implicit switch fallthrough** → use [[fallthrough]], break, or return. Consecutive empty cases are fine. [ES.78]
- **Multi-allocation in one expr** → `f(shared_ptr(new X), shared_ptr(new Y))` leaks if second throws. Use make_shared/make_unique. [R.13, C.151]
- **Aliased smart pointer deref** → pin with local copy before deref if called functions might reseat. [R.37]
- **Non-const global variable** → hidden dependency, untestable, data-race-prone. const/constexpr globals OK. [I.2]
- **Cyclic shared_ptr** → two objects holding shared_ptr to each other never reach refcount 0 → memory leak. Break cycles with weak_ptr. [R.24]

## MEDIUM (suggest, don't block)

- **Narrowing conversion** → `int x = double_val;` loses info. Prefer {} init or gsl::narrow_cast. [ES.46]
- **Signed/unsigned mixing** → surprising results in arithmetic/comparison. OK for bit manipulation. [ES.100, ES.101, ES.102]
- **Missing explicit** on single-arg ctor → accidental implicit conversion. OK for copy/move ctors and intentional conversions. [C.46]
- **[=] capture in member function** → captures this by pointer, not data members. All member access goes through the pointer — dangles if lambda outlives the object, same as [&]. C++20 deprecated implicit this capture via [=]. Use `[=, this]` to explicitly capture the pointer, or `[=, *this]` (C++17) to copy the entire object. [F.54]
- **return std::move(local)** → prevents NRVO. Just `return local;`. Exception: returning data member, different type needing conversion, or rvalue-ref parameter. [F.48]
- **Smart pointer param when function doesn't own** → take T*/T& instead. Smart pointer param = ownership signal. [R.30, F.7]
- **shared_ptr where unique_ptr suffices** → prefer unique_ptr for single owner. [R.20, R.21]
- **swap not noexcept** → standard library relies on non-throwing swap. [C.85]
- **enum instead of enum class** → implicit int conversion, namespace pollution. [Enum.3]
- **Missing override** → compiler can't catch signature mismatch. Use override, not virtual+override. [C.128]
- **Macros for constants/functions** → use constexpr/templates. OK for include guards, #ifdef platform. [ES.31]

## Decision Trees

### Raw pointer
```text
From new/malloc?
  → Yes: Flag — use smart pointer

Function param?
  → stores/deletes it?
      → Yes: Flag — take smart pointer
  → doesn't store/delete?
      → nullable intent?
          → Yes: OK — T* as non-owning observer
      → never null?
          → Suggest T& instead

Class member?
  → class deletes it?
      → Yes: Flag — use smart pointer
  → observing only?
      → OK
```

### Exception handling
```text
Catch clause?
  → by value?
      → Flag: catches by value can slice — use catch(const E&)
  → by reference?
      → OK

In catch block, rethrowing?
  → throw e;
      → Flag: copies and may slice — use bare throw;
  → throw;
      → OK: rethrows original exception object
```
