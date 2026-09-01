# go-discount-checkout

A production-quality, fully-featured discount and checkout engine written in idiomatic Go. This project is a **design-patterns tutorial disguised as a real e-commerce system** — every design decision is documented in the source and this README walks you through each layer from first principles.

> **Go version:** 1.24 (uses `iter.Seq` / `iter.Seq2` range-over-func from Go 1.23; `slices.SortStableFunc` from Go 1.21; `log/slog` from Go 1.21)

---

## Table of Contents

1. [What this project is](#what-this-project-is)
2. [Architecture overview](#architecture-overview)
3. [The problem being solved](#the-problem-being-solved)
4. [Step-by-step layer walkthrough](#step-by-step-layer-walkthrough)
   - [Step 1 — `pkg/money` — the Money Value Object](#step-1--pkgmoney--the-money-value-object)
   - [Step 2 — `internal/domain` — the DDD Aggregate Root](#step-2--internaldomain--the-ddd-aggregate-root)
   - [Step 3 — `internal/spec` — the Specification Pattern](#step-3--internalspec--the-specification-pattern)
   - [Step 4 — `internal/discount` — the Strategy Pattern](#step-4--internaldiscount--the-strategy-pattern)
   - [Step 5 — `internal/promotion` — composing When + What](#step-5--internalpromotion--composing-when--what)
   - [Step 6 — `internal/audit` — structured logging abstraction](#step-6--internalaudit--structured-logging-abstraction)
   - [Step 7 — `internal/checkout` — the Orchestrator](#step-7--internalcheckout--the-orchestrator)
   - [Step 8 — `internal/pipeline` — concurrent batch processing](#step-8--internalpipeline--concurrent-batch-processing)
5. [Design patterns used](#design-patterns-used)
6. [Go-specific patterns & idioms](#go-specific-patterns--idioms)
7. [Concurrency model](#concurrency-model)
8. [Running the demo](#running-the-demo)
9. [Running the tests](#running-the-tests)
10. [Extending the system](#extending-the-system)

---

## What this project is

This project simulates the promotion and checkout engine of a supermarket loyalty system (think Tesco Clubcard). It handles:

| Concern | Solution |
|---|---|
| Safe money arithmetic | `Money` value object (integer pence, HALF_UP rounding) |
| Shopping basket management | `Order` DDD Aggregate Root (thread-safe) |
| "When does a deal apply?" | `Specification[T]` — composable generic predicates |
| "How much does it save?" | `Discount` — Strategy interface |
| Named promotions (deals) | `Promotion` — binds When + What + Priority + ExclusiveGroup |
| Resolving conflicting deals | `ConflictPolicy` — pluggable function in `Checkout` |
| Orchestrating everything | `Checkout` — pure orchestrator, no business logic |
| Processing 1000s of baskets | `BatchProcessor` — bounded goroutine Worker Pool |
| Audit trail | `audit.Logger` — slog-compatible structured logging |

---

## Architecture overview

```
discount-checkout/
├── pkg/
│   └── money/
│       └── money.go          # Money Value Object (integer pence, immutable)
├── internal/
│   ├── domain/
│   │   ├── customer.go       # Customer value object + CustomerTier enum
│   │   ├── order.go          # Order — DDD Aggregate Root (thread-safe basket)
│   │   └── receipt.go        # Receipt — checkout output DTO + AppliedDiscount
│   ├── spec/
│   │   ├── spec.go           # Specification[T] interface + And/Or/Not/Always/Never
│   │   ├── contains_sku.go   # Rule: basket contains ≥N of a SKU
│   │   ├── has_coupon.go     # Rule: coupon code applied to basket
│   │   ├── min_spend.go      # Rule: basket subtotal ≥ threshold
│   │   ├── min_tier.go       # Rule: customer loyalty tier ≥ required
│   │   └── member_only.go    # Rule: customer is a loyalty member
│   ├── discount/
│   │   ├── discount.go       # Discount interface + Func adapter
│   │   ├── percent_off.go    # X% off the entire basket subtotal
│   │   ├── percent_off_sku.go# X% off a specific SKU line total
│   │   ├── fixed_amount_off.go # Fixed £N off (capped at subtotal)
│   │   ├── bogof.go          # Buy X Get Y Free (multi-buy)
│   │   └── stacked.go        # Composite — combines multiple discounts
│   ├── promotion/
│   │   ├── promotion.go      # Promotion struct + Evaluate (comma-ok)
│   │   └── factory.go        # Convenience constructors (BOGOF, MemberPercentOff…)
│   ├── audit/
│   │   └── audit.go          # Logger interface + SlogLogger + InMemoryLogger
│   ├── checkout/
│   │   ├── options.go        # Option (Functional Options) + ConflictPolicy
│   │   └── checkout.go       # Checkout orchestrator — Total() pipeline
│   └── pipeline/
│       └── batch.go          # BatchProcessor — Worker Pool + Fan-Out/Fan-In
└── cmd/
    └── checkout/
        └── main.go           # Runnable demo wiring all layers together
```

---

## The problem being solved

A supermarket runs many simultaneous promotions. Given a customer's basket, the system must:

1. **Evaluate** all active promotions against the basket.
2. **Stack** compatible promotions (e.g. a BOGOF + a member discount can both apply).
3. **Resolve conflicts** — if two promotions are in the same "exclusive group" (e.g. two competing basket-wide percentage offers), only the best one wins.
4. **Cap savings** so the customer never pays a negative amount.
5. **Produce a Receipt** with a full audit trail.
6. **Scale** — process thousands of baskets concurrently without goroutine leaks.

The central insight is to **separate three concerns**:

```
WHEN does a promotion apply?  →  Specification[*Order]
HOW MUCH does it save?        →  Discount.Apply(*Order) Money
WHO orchestrates them?        →  Checkout.Total(ctx, *Order) Receipt
```

---

## Step-by-step layer walkthrough

### Step 1 — `pkg/money` — the Money Value Object

**Problem:** `float64` arithmetic loses precision. `0.1 + 0.2 != 0.3` in IEEE 754. This is a showstopper for financial calculations.

**Solution:** Store money as `int64` **minor units** (pence, cents):

```go
type Money struct {
    cents    int64  // e.g. 1250 represents £12.50
    currency string
}
```

Every operation returns a **new** `Money` value (immutable by convention, value receiver):

```go
price := money.GBP("12.50")       // £12.50 = 1250 pence
total := price.Mul(3)              // £37.50 (never float precision loss)
saving := total.Percent(10)        // £3.75 (HALF_UP retail rounding)
payable, _ := total.Sub(saving)    // £33.75
```

**Why HALF_UP rounding for percentages?**

```go
// (cents * pct + 50) / 100
// £0.725 → rounds up to £0.73 (retailer rounds to customer's disadvantage)
discountCents := (m.cents*int64(pct) + 50) / 100
```

**Why `Min` for capping?**

```go
// Fixed £20 off cannot exceed the subtotal — customer never pays negative.
return f.Amount.Min(order.Subtotal())
```

**Go idioms demonstrated:**
- `Make the zero value useful` — `Money{}` safely defaults to `GBP 0.00`
- `Must*` variants — `money.GBP("12.50")` panics on parse error (safe in tests/init)
- Full JSON + TextMarshaler/TextUnmarshaler support for persistence/APIs

---

### Step 2 — `internal/domain` — the DDD Aggregate Root

**`Customer`** is a pure value object:

```go
type Customer struct {
    ID       string
    IsMember bool
    Tier     CustomerTier   // Standard, Silver, Gold
}

guest  := domain.Guest("cust-42")
member := domain.Member("cust-007", domain.TierGold)
```

`CustomerTier` uses `iota` starting from 0 so the zero value is `TierStandard` — the safe default for any uninitialised `Customer{}`.

**`Order`** is the DDD **Aggregate Root** — the central object that guards invariants:

```go
order := domain.GBPOrder(shopper).
    MustAddLine("PASTE", "Colgate Toothpaste", money.GBP("1.50"), 6).
    MustAddLine("STEAK", "Prime Ribeye Steak", money.GBP("12.00"), 5).
    MustRedeemCoupon("SAVE5")
```

Key design decisions:

| Decision | Reason |
|---|---|
| `sync.RWMutex` inside `Order` | Safe for concurrent reads by multiple promotion evaluators |
| `Lines()` returns `slices.Clone(...)` | Defensive copy — callers cannot corrupt internal state |
| `HasCoupon` normalises to upper-case | Case-insensitive coupon codes without extra complexity |
| `MustAddLine` / `MustRedeemCoupon` | Panic-on-error for fluent chaining in tests; explicit `AddLine` for production |
| Go 1.23+ `LinesSeq()`, `CouponsSeq()` | Zero-allocation iteration without materialising intermediate slices |

**`Receipt`** is a pure DTO (Data Transfer Object) produced by `Checkout`:

```go
type Receipt struct {
    Lines        []OrderLine
    Subtotal     money.Money
    Discounts    []AppliedDiscount  // audit trail of what was applied
    TotalSavings money.Money
    Payable      money.Money
}
```

---

### Step 3 — `internal/spec` — the Specification Pattern

**Problem:** "Does this promotion apply to this basket?" is a business rule. If you embed it as `if` statements inside the discount logic or the orchestrator, you get an unmaintainable tangle.

**Solution:** DDD Specification Pattern — every eligibility rule is its own composable object.

```go
type Specification[T any] interface {
    IsSatisfiedBy(candidate T) bool
}
```

The built-in specifications:

| Specification | Question it answers |
|---|---|
| `ContainsSKU("PASTE", 3)` | Does the basket have ≥3 tubes of toothpaste? |
| `HasCoupon("SAVE5")` | Has the customer applied coupon SAVE5? |
| `MinimumSpend(GBP("50"))` | Is the basket subtotal at least £50? |
| `MinimumTier(TierGold)` | Is the customer a Gold-tier member? |
| `MemberOnly{}` | Is the customer a loyalty member at all? |

Compose them freely with generic combinators:

```go
// Gold VIP member condition:
goldVIP := spec.And[*domain.Order](
    spec.NewMemberOnly(),
    spec.NewMinimumTier(domain.TierGold),
)

// Coupon + minimum spend:
couponRule := spec.And[*domain.Order](
    spec.NewHasCoupon("SAVE5"),
    spec.NewMinimumSpend(money.GBP("30.00")),
)

// Either condition works:
either := spec.Or[*domain.Order](goldVIP, couponRule)
```

**Why generics?** The same combinators (`And`, `Or`, `Not`) work for any candidate type — `*Order`, `Customer`, `Product`, etc. — without code duplication.

**The `Func` adapter** lets plain functions satisfy the interface without a struct:

```go
// Inline rule without creating a named type:
customRule := spec.Func[*domain.Order](func(order *domain.Order) bool {
    return order.ItemCount() >= 5
})
```

---

### Step 4 — `internal/discount` — the Strategy Pattern

**Problem:** There are many ways to calculate a saving. BOGOF, percentage off, fixed amount off, percentage off a specific SKU... hard-coding switch statements breaks Open/Closed.

**Solution:** Strategy Pattern — every calculation is an interchangeable object behind one interface:

```go
type Discount interface {
    Apply(order *domain.Order) money.Money
}
```

Built-in strategies:

| Type | What it computes |
|---|---|
| `PercentOff{Percent: 10}` | 10% of the basket subtotal |
| `PercentOffSKU{SKU: "WINE", Percent: 20}` | 20% of the WINE line total |
| `FixedAmountOff{Amount: GBP("5")}` | £5 off, capped at subtotal |
| `BOGOF{SKU: "PASTE", PayFor: 2, FreePerGroup: 1}` | Buy 2 Get 1 Free |
| `StackedDiscount{parts: [...]}` | Composite of multiple discounts |

**BOGOF calculation:**

```go
groupSize := b.PayFor + b.FreePerGroup   // 2 + 1 = 3
freeUnits := (qty / groupSize) * b.FreePerGroup
// qty=6, groupSize=3 → freeUnits=2 → saving = 2 × unit price
```

**`StackedDiscount`** is the **Composite Pattern** — it combines multiple strategies into one:

```go
combo := discount.Stack(
    discount.NewPercentOff(5),
    discount.NewFixedAmountOff(money.GBP("2.00")),
)
// saving = 5% + £2.00, capped at subtotal
```

**Why cap at subtotal?**

```go
return totalSaving.Min(order.Subtotal())
```

A stack of discounts must not make the customer's payable amount go negative.

---

### Step 5 — `internal/promotion` — composing When + What

A `Promotion` is the glue that binds a `Specification` (eligibility) with a `Discount` (saving formula):

```go
type Promotion struct {
    Name           string
    When           spec.Specification[*domain.Order]   // eligibility rule
    What           discount.Discount                    // saving formula
    Priority       int                                  // lower = higher precedence
    ExclusiveGroup string                               // empty = stackable
}
```

**Policy as Data:** whether a promotion is stackable or exclusive is a field, not a type hierarchy. This allows the `Checkout` orchestrator to handle both uniformly.

```go
// Stackable: applies alongside other promotions.
promo1 := promotion.New("BOGOF Toothpaste", spec.Contains("PASTE"), discount.BuyTwoGetOneFree("PASTE"))

// Exclusive: competes within group "basket-wide"; only the best wins.
promo2 := promotion.NewWithPolicy("10% off £50+", minSpend, percentOff10, 0, "basket-wide")
promo3 := promotion.NewWithPolicy("Clubcard 5%",  memberOnly,  percentOff5,  1, "basket-wide")
```

**`Evaluate`** uses the comma-ok idiom — clean at the call site:

```go
if applied, ok := promo.Evaluate(order); ok {
    // applied is an AppliedDiscount with the name and saving amount
}
```

**The factory functions** (`promotion.BOGOF`, `promotion.MemberPercentOff`, `promotion.GoldFixedOff`, …) are named constructors that compose spec + discount — they are the readable, production-facing API:

```go
engine := checkout.With(
    promotion.BOGOF("BOGOF Toothpaste", "PASTE"),
    promotion.PercentOffOver("10% over £50", 10, money.GBP("50.00")),
    promotion.MemberPercentOff("Clubcard 5%", 5),
    promotion.GoldFixedOff("Gold £20 Off", money.GBP("20.00")),
    promotion.CouponFixedOff("SAVE5 Voucher", "SAVE5", money.GBP("5.00"), money.GBP("30.00")),
)
```

---

### Step 6 — `internal/audit` — structured logging abstraction

**Problem:** The `Checkout` needs to log what it does, but it must not depend on a specific logger (coupling to `slog`, `zap`, or `logrus` directly makes testing hard).

**Solution:** A minimal single-method interface:

```go
type Logger interface {
    Log(ctx context.Context, level slog.Level, msg string, args ...any)
}
```

Three implementations:

| Implementation | When to use |
|---|---|
| `audit.NoOp()` | Default — no logging, zero overhead |
| `audit.NewSlogLogger(l)` | Production — wraps `log/slog` |
| `audit.NewInMemoryLogger()` | Tests — assert what was logged |

```go
// Production:
auditLogger := audit.NewSlogLogger(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
engine := checkout.New(checkout.WithLogger(auditLogger))

// Tests — verify the audit trail:
logger := audit.NewInMemoryLogger()
engine := checkout.New(checkout.WithLogger(logger))
engine.MustTotal(order)
assert.True(t, logger.ContainsMessage("applied promotion"))
assert.Equal(t, 3, len(logger.FindByLevel(slog.LevelInfo)))
```

---

### Step 7 — `internal/checkout` — the Orchestrator

`Checkout` is the pure orchestrator. It contains **no business rules** of its own — rules live in `Specification` and `Discount`. `Checkout` only manages the pipeline:

```
evaluate all promotions → separate stackable vs exclusive → resolve conflicts
→ sort for deterministic output → sum savings → apply caps → build Receipt
```

**Functional Options Pattern:**

```go
engine := checkout.New(
    checkout.WithPromotions(promos...),
    checkout.WithLogger(logger),
    checkout.WithMaxSavingsCap(money.GBP("30.00")),  // hard limit on total savings
    checkout.WithConflictPolicy(checkout.BestSavingWins),
)
```

Adding new configuration never changes the `New` signature — backward-compatible forever.

**The `Total` pipeline in detail:**

```go
func (c *Checkout) Total(ctx context.Context, order *domain.Order) (domain.Receipt, error) {
    // 1. Thread-safe snapshot of engine state (slices.Clone of promotions).
    // 2. Fail fast: check ctx.Err() before doing any work.
    // 3. Evaluate all promotions → collect eligible candidates.
    // 4. Separate stackable / exclusive; resolve exclusive groups via ConflictPolicy.
    // 5. slices.SortStableFunc by Priority, then Name (deterministic audit order).
    // 6. Sum savings; cap at subtotal; apply optional MaxSavingsCap.
    // 7. Build and return Receipt.
}
```

**Thread-safe snapshot pattern** (step 1): the engine takes a read lock, clones the promotions slice, then releases the lock *before* the expensive evaluation loop. This means promotions can be added at runtime without blocking ongoing checkouts:

```go
c.mu.RLock()
promos := slices.Clone(c.promotions)
c.mu.RUnlock()
// Now work on the snapshot — no lock held during evaluation.
```

**ConflictPolicy** is a pluggable function type:

```go
type ConflictPolicy func(a, b evaluatedCandidate) evaluatedCandidate

// Default: pick the one with the larger saving.
func BestSavingWins(a, b evaluatedCandidate) evaluatedCandidate { ... }

// Custom: always prefer explicitly prioritised promotions.
myPolicy := func(a, b checkout.ConflictPolicy) { ... }
engine := checkout.New(checkout.WithConflictPolicy(myPolicy))
```

---

### Step 8 — `internal/pipeline` — concurrent batch processing

**Problem:** Processing 10,000 baskets sequentially is too slow. Spawning one goroutine per basket wastes memory (each goroutine costs ~2 KB stack).

**Solution:** Bounded Worker Pool — exactly `N` goroutines consume from a job channel (Fan-Out), results flow back through a result channel (Fan-In):

```
orders → jobsChan → [worker 1] ──┐
                  → [worker 2] ──┤── resultsChan → results[]
                  → [worker 3] ──┤
                  → [worker 4] ──┘
```

```go
batchProcessor := pipeline.NewBatchProcessor(engine, 4) // 4 workers
results, err := batchProcessor.ProcessBatch(ctx, batchOrders)
```

**Why `sync/atomic` for `ProcessedCount`?**

```go
type BatchProcessor struct {
    processedOrders atomic.Int64  // lock-free counter
}
```

`atomic.Int64` is cheaper than a mutex for a simple counter — no contention on a hot path.

**Why close channels with a goroutine and WaitGroup?**

```go
go func() {
    wg.Wait()          // block until all workers are done
    close(resultsChan) // only then signal the collector
}()
```

This prevents the main goroutine from blocking on `resultsChan` while workers are still running, and guarantees the channel is closed exactly once after all workers exit — no goroutine leaks.

**`ProcessStream`** demonstrates Go 1.23+ iterator integration:

```go
// Memory-efficient: processes an infinite stream of orders without buffering all at once.
for result := range batchProcessor.ProcessStream(ctx, ordersIterator) {
    // handle each result as it arrives
}
```

---

## Design patterns used

| Pattern | Package | Purpose |
|---|---|---|
| **Value Object** | `pkg/money` | Immutable `Money` with type-safe arithmetic |
| **DDD Aggregate Root** | `internal/domain` | `Order` guards all basket invariants |
| **Specification** | `internal/spec` | Composable eligibility rules (`And`, `Or`, `Not`) |
| **Strategy** | `internal/discount` | Interchangeable saving algorithms |
| **Composite** | `internal/discount` | `StackedDiscount` combines multiple strategies |
| **Functional Options** | `internal/checkout` | `Option func(*Checkout)` for zero-breaking-change config |
| **Policy as Data** | `internal/promotion` | `ExclusiveGroup` field replaces a class hierarchy |
| **Factory** | `internal/promotion` | Named constructors (`BOGOF`, `MemberPercentOff`…) |
| **Adapter (Func)** | `spec`, `discount`, `audit` | `Func` types let plain functions satisfy interfaces |
| **Observer / Audit** | `internal/audit` | `Logger` interface decouples logging from business logic |
| **Worker Pool** | `internal/pipeline` | Bounded goroutine pool for concurrent batch processing |

---

## Go-specific patterns & idioms

| Idiom | Where | Why |
|---|---|---|
| **Make the zero value useful** | `Money{}`, `Customer{}`, `audit.NoOp()` | Safe defaults without explicit initialisation |
| **Must* variants** | `MustAddLine`, `MustTotal`, `money.GBP()` | Panic-on-error for tests and init code |
| **Comma-ok** | `Promotion.Evaluate` | Idiomatic `(T, bool)` return for operations that may not produce a value |
| **`slices.Clone` defensive copy** | `Order.Lines()`, `Checkout.Promotions()` | Callers cannot corrupt internal state |
| **`slices.SortStableFunc`** | `Checkout.Total()` | Type-safe, reflection-free, deterministic sorting (Go 1.21+) |
| **`sync/atomic.Int64`** | `BatchProcessor` | Lock-free counter for observability metrics |
| **`iter.Seq` / `iter.Seq2`** | `Order`, `Receipt`, `InMemoryLogger` | Zero-allocation range-over-func iteration (Go 1.23+) |
| **Context propagation** | `Checkout.Total`, `BatchProcessor` | Cancellation and timeout support throughout |
| **`errors.Is` sentinel errors** | `money`, `domain`, `checkout` | Distinguishable error kinds without type assertions |
| **Single-method interfaces** | `Discount`, `Specification`, `Logger` | Minimal contracts; `Func` adapters for free |

---

## Concurrency model

| Type | Synchronisation | Reason |
|---|---|---|
| `Order` | `sync.RWMutex` | Multiple promotion evaluators may read concurrently; writes (AddLine) are exclusive |
| `Checkout` | `sync.RWMutex` + snapshot | Engine state (promotions list) snapshotted under lock; evaluation runs lock-free |
| `BatchProcessor` | Channels + `sync.WaitGroup` | Fan-Out/Fan-In; WaitGroup prevents goroutine leaks |
| `InMemoryLogger` | `sync.RWMutex` | Concurrent appends from multiple goroutines |
| `BatchProcessor.processedOrders` | `sync/atomic.Int64` | Lock-free counter on the hot path |

**Deadlock prevention in `Checkout`:** the engine acquires `RLock`, clones state, releases the lock, then calls promotion evaluators. No lock is held during evaluation — promotions cannot re-enter `Checkout` and deadlock.

---

## Running the demo

```bash
cd cmd/checkout
go run .
```

Expected output (JSON audit lines from slog, then the formatted receipt):

```
==========================================================
   Discount & Checkout Engine (Modern Go Native)
==========================================================

[Basket Summary] Customer: cust-gold-007 | Lines: 12 | Gross Total: GBP 104.00

--------------------- SHOPPING RECEIPT ----------------------
 - Colgate Toothpaste     6 x GBP 1.50 = GBP 9.00
 - Prime Ribeye Steak     5 x GBP 12.00 = GBP 60.00
 - Chateau Margaux        1 x GBP 35.00 = GBP 35.00
--------------------------------------------------------------
 Subtotal:                   GBP 104.00
 Applied Discounts:
   * BOGOF Toothpaste              -GBP 3.00
   * 10% over £50                  -GBP 10.40
   * Clubcard 5%                   -GBP 5.20
   * Gold £20 Off                  -GBP 20.00
   * Voucher SAVE5 (£5 off)        -GBP 5.00
 Total Savings:              -GBP 43.60   (capped at subtotal)
 Amount Payable:              GBP 60.40
 Summary: Subtotal: GBP 104.00 | Savings: -GBP 43.60 (5 promos) | Payable: GBP 60.40
==========================================================

[Concurrent Pipeline Demo] Processing 20 orders across 4 goroutine workers...
✅ Done: 20 baskets processed concurrently. Total processed orders: 20
```

---

## Running the tests

```bash
go test ./...                # all packages
go test -race ./...          # with the data race detector
go test -cover ./...         # with coverage report
go test ./internal/checkout/... -v  # verbose checkout tests
```

Test packages cover:
- `pkg/money` — arithmetic, rounding, serialisation, edge cases
- `internal/domain` — order/customer/receipt invariants, concurrent access
- `internal/spec` — all specification types + And/Or/Not composition
- `internal/discount` — all discount strategies + stacking + capping
- `internal/promotion` — evaluation, exclusive group resolution
- `internal/checkout` — full end-to-end scenarios with multiple promotions
- `internal/pipeline` — concurrent batch processing, context cancellation
- `internal/audit` — in-memory logger assertions

---

## Extending the system

**Add a new eligibility rule:**
```go
// internal/spec/weekend_only.go
type WeekendOnly struct{}

func (w WeekendOnly) IsSatisfiedBy(order *domain.Order) bool {
    day := time.Now().Weekday()
    return day == time.Saturday || day == time.Sunday
}

// Compose with existing specs:
weekendMember := spec.And[*domain.Order](WeekendOnly{}, spec.NewMemberOnly())
```

**Add a new discount formula:**
```go
// internal/discount/loyalty_points.go — discount based on loyalty points balance
type LoyaltyPointsDiscount struct{ PointsBalance int }

func (l LoyaltyPointsDiscount) Apply(order *domain.Order) money.Money {
    // £1 per 100 points
    return money.New(int64(l.PointsBalance/100)*100, order.Currency())
}
```

**Add a new promotion (zero changes to existing code):**
```go
engine.AddPromotions(
    promotion.New(
        "Weekend Double Points",
        spec.And[*domain.Order](WeekendOnly{}, spec.NewMemberOnly()),
        LoyaltyPointsDiscount{PointsBalance: customer.LoyaltyPoints},
    ),
)
```

**Add a new conflict policy:**
```go
// Always prefer promotions with lower Priority value, regardless of saving size.
priorityFirst := func(a, b checkout.ConflictPolicy) checkout.ConflictPolicy {
    if a.promo.Priority <= b.promo.Priority { return a }
    return b
}
engine := checkout.New(checkout.WithConflictPolicy(priorityFirst))
```

**Plug in a different repository / logger:**
```go
// Any type satisfying audit.Logger works:
datadogLogger := audit.Func(func(ctx context.Context, level slog.Level, msg string, args ...any) {
    datadog.Log(level.String(), msg, args...)
})
engine := checkout.New(checkout.WithLogger(datadogLogger))
```
