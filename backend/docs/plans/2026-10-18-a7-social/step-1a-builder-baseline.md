> 本檔是 2026-10-23 決定「共用可見性述詞的讀取用 go-jet」（Will）時的基準報告，原樣收進版控當依據。文中的 `scratchpad/…` 路徑（spike 程式、go 專案調查、prior report）是當時 session 的草稿，不在版控裡；結論與約束表已寫進 `docs/adr/2026-09-27-module-internal-structure.md` 的修訂，實作在 `internal/modules/social/internal/{visibility,reads,counted}`。

# Shared visibility predicates in Go + Postgres: builder baseline, spike, ledger, recommendation

Written 2026-10-23, from scratch, building on `scratchpad/predicate-baseline.md`, which recommended DB-side inlinable SRFs (set-returning SQL functions).
Evidence tags:
- **[measured]**: I ran it this session, on PG 18 (postgres:18-alpine on tmpfs, fsync off), 10^5 posts, 714k comments, one hot post with 20k top-level comments, a single client.
- **[read-source]**: I opened the library or project source. Library sources come from the Go module cache; project sources come from the survey subagent's pinned clones, see `scratchpad/survey/go-projects.md`.
- **[docs]**: official docs or issue-tracker text.
- **[inferred]**: reasoning from the above.
- **[unverified]**: from memory.

Spike code: `scratchpad/builder-spike/`. It is its own Go module. Container `builder-spike-pg` on port 55901 is still running; `sql/00..03` rebuild it.

## 0. Premise check

- The code that would change is social's `db/objects.sql` views, the 24 queries that carry the rule, and `readpolicy_test.go` in the a7-1a worktree. Nothing has shipped, so there is no compatibility constraint (repo rule: no compatibility constraints before launch).
- Count of affected queries: 24 of social's 100 sqlc queries mention `published_posts`, `visible_*`, `pub_exposable_stores` or `pub_inactive_buyers`. That includes 3 recount queries [measured: awk over `db/queries/*.sql`]. The other ~76 are writes, locks, purge, media refs and owner reads, and they do not need composition.
- Why the current mechanism exists. sqlc cannot compose fragments (sqlc #2244, open since 2023-04 [docs]). The monolith rules forbid social DB objects that reference `seller.*` or `buyer.*`, because a module's migration must apply alone to an empty DB. So the rule is either re-spelled in every query and checked by counting, or pushed into DB functions, which is what the prior report recommended.

## 1. Industry practice in Go

### 1.1 Libraries

| Option | How a shared predicate is expressed | Type safety | pgx-native (the repo's `db.DBTX`) | Status | Evidence |
|---|---|---|---|---|---|
| **go-jet/jet** v2.16.1 | `func(p *table.PostsTable, …) BoolExpression`. Generated table and column types come from the live DB. | Operands are typed (`IntegerExpression.EQ(IntegerExpression)`). A renamed column breaks the build: renaming `post_stats.like_count` and regenerating gave `s.LikeCount undefined` [measured]. Scanning is by name at run time (below) | **No**. Its QRM scanner (jet's built-in row mapper) needs `database/sql` (`qrm.Queryable` returns `*sql.Rows`) [read-source qrm/db.go]. Use `.Sql()` and scan with pgx instead | 3.8k★, pushed 2026-10-12 | [read-source][measured] |
| **stephenafamo/bob** v0.50.0 | `bob.Expression` or `bob.Mod[*dialect.SelectQuery]`. It can also generate sqlc-like typed queries from `.sql` files, and you can add mods to them (`psql.Select(GeneratedQuery(...), sm.Where(pred))`) | Column helpers are untyped `psql.Expression`. Rows generated from SQL files are typed and scanned by index. Renaming a column made generation fail (`prepare: column s.like_count does not exist`) [measured] | Yes (`drivers/pgx`, wraps a `pgx.Tx`) [read-source] | 1.8k★, pushed 2026-10-06. Few public adopters; the survey found no notable one using shared predicates [unverified] | [read-source][measured] |
| squirrel / goqu | `func(sq.SelectBuilder) sq.SelectBuilder` helpers (the Mattermost style) | None: strings everywhere | Yes, through `ToSql()` | squirrel was last pushed 2024-04, goqu 2024-05 (both are maintenance-mode) | [read-source Mattermost][gh api] |
| sqlc + run-time fragment splice (Coder) | A `-- @authorize_filter` marker in the `.sql`; `strings.ReplaceAll` puts a Rego-compiled SQL fragment there at run time (`modelqueries.go:1266`) | Reuses sqlc's SQL constant and its param and row types, but the `rows.Scan` is **hand-copied**. An AST-diff test (`TestCustomQueriesSyncedRowScan`) guards it. No test checks that every query has the marker; a missing marker is a run-time error | Yes | Coder is the one large sqlc shop | [read-source] |
| sqlc + preprocessor (Go templates or Jinja) | Text macros before sqlc | sqlc-level | Yes | Proposed by the sqlc maintainer in #2244 (2023-09) and never shipped. No notable adopter found | [docs][unverified negative] |
| ent privacy / interceptors | `intercept.TraverseFunc` adds a WHERE to every traversal of a schema | Generated, typed | Own driver | theopenlane/core uses it (`mixin_orgowned.go:233`) | [docs][read-source] |
| DB views or functions (status quo, prior report) | View or inlinable SRF in the owning schema | sqlc-typed | n/a | Coder uses `*_expanded` views and Gitaly uses `valid_primaries` views, but only for **projection or health** predicates, never for per-viewer visibility | [read-source] |

### 1.2 What well-known Go projects do (details in `scratchpad/survey/go-projects.md`)

- **Mattermost** (squirrel) [read-source]:
  - Free functions return predicates: `applyViewRestrictionsFilter` (`user_store.go:2163`, 10 call sites) and the per-viewer `burnOnReadVisibleCondition` (`post_store.go:1920`).
  - The code comment says they filter in SQL "instead of returning a window full of posts that are later filtered out".
- **Gitea / Forgejo** (xorm `builder.Cond`) [read-source]:
  - `AccessibleRepositoryCondition` (`repo_list.go:661`) and its subquery form `AccessibleRepoIDsQuery` are reused by activities, git, LFS and repo search.
  - Blocking is applied only on write paths (follow, star, mention), not as a read filter. This was checked in `models/` only.
- **Grafana** [read-source]: `accesscontrol.Filter` (`filter.go:42`) turns the permissions already loaded for the user into `col IN (…)`, with an allow-list of column names.
- **Coder** (sqlc) [read-source]:
  - Run-time splice, as in the table.
  - `dbauthz` wraps every Store method, and a reflection test fails if a method has no RBAC test.
  - It filters in SQL at 22 sites and post-filters in Go at 36.
- **Gitaly / Praefect** [read-source]: raw SQL constants plus views, with no user-visibility filtering.
- **fivenet** (jet, MySQL, 26★) [read-source]: `VisibleIDsByConditionQuery` returns a typed CTE or subquery that 7+ stores reuse. It is the only jet user found with a shared visibility predicate.

**What the industry does [inferred from the rows above plus the prior report's Lemmy, Discourse, Mastodon, GitLab and Django evidence]:**
1. The visibility rule is a composable value in the application's query layer (Diesel fn, Rails scope, Django manager, `builder.Cond`, squirrel helper, ent interceptor). It is applied in SQL for every list, so pagination and indexes keep working.
2. Completeness comes from a single choke point plus a test (dbauthz, ent interceptors), not from convention.
3. No surveyed project puts per-viewer visibility in a DB view or function. DB-side objects are used for shared projections and health checks.
4. sqlc shops that need this either splice text at run time (Coder) or give up on composition (CASE toggles).

## 2. Spike: go-jet (primary) and bob (secondary) on the real social schema

Setup [measured]:
- Real `social/db/schema.sql` + `objects.sql`.
- Stub `seller` and `buyer` schemas whose `pub_*` functions have the real signatures (`BEGIN ATOMIC`, each module in its own schema).
- New `seller.pub_active_content_takedowns(at)` and a spike-only `social.blocks` table, standing in for the future takedowns and blocks.
- Seed: 10^5 posts (10 % drafts, 2 % deleted), 714k comments, 1 % hidden, 3 % inactive buyers, 488 post takedowns. Viewer 1 blocks about 200 buyers, is blocked by about 95, and is blocked by 20 stores.

### 2.1 Shared predicates (`visjet/vis.go`)

- `ReadablePosts(p, at, viewer)` = `PostPublished(p)` (inline `'PUBLISHED'`) AND `sellerpub.StoreExposable(at, p.StoreID)` AND NOT `sellerpub.ContentTakenDown(at,"POST",p.ID)`. When there is a viewer, add NOT store-blocked-viewer.
- `CountedComments(c, at)`: viewer-independent. Not deleted, no active hide, NOT `buyerpub.Inactive(c.ActorID, c.ActorKind='BUYER')`, not taken down.
- `ViewableComments(c, at, viewer)` = `CountedComments` plus two separate block NOT EXISTS, with the `actor_kind='BUYER'` guard pushed inside each subquery so they stay anti-joins.
- `viewer == nil` leaves the block SQL out entirely. The statement text differs, so there is no `$v IS NULL OR …` toggle to spoil the generic plan, a plan built once without the parameter values.
- `at` is an expression, not a `time.Time`. The same predicate then renders as `$n` in reads and as `NOW()` in the generated trigger function (§2.6).
- `pub/sellerpub` and `pub/buyerpub` are the only places that spell `seller.`/`buyer.` SQL. They use jet's `CustomExpression` because jet has no table-function source [read-source]. In the repo they would be generated from the owner's `pub.sql`.

### 2.2 Generated SQL (the feed, `out/explain.txt`)

```sql
SELECT p.id AS "p.id", … FROM social.posts AS p INNER JOIN social.post_stats AS s ON (s.post_id = p.id)
WHERE ((p.status = 'PUBLISHED') AND (p.deleted_at IS NULL)
  AND EXISTS (SELECT 1 FROM seller.pub_exposable_stores($1::timestamp with time zone) e WHERE e.store_id = p.store_id)
  AND NOT EXISTS (SELECT 1 FROM seller.pub_active_content_takedowns($2::timestamp with time zone) t WHERE t.target_kind = 'POST' AND t.target_id = p.id)
  AND NOT EXISTS (SELECT $3::integer FROM social.blocks AS sb WHERE sb.blocker_kind = 'STORE' AND sb.blocker_id = p.store_id AND sb.blocked_buyer_id = $4::bigint))
  AND (ROW(p.published_at, p.id) < ROW($5::timestamp with time zone, $6::uuid))
ORDER BY p.published_at DESC, p.id DESC LIMIT $7;
```

It has the same shape a person would write. The top-level comment and trending SQL are in `out/explain.txt`.
- Top-level comments: the container check is one `EXISTS (… cp.id = $post AND ReadablePosts(cp))`. Placeholders are `v.visible OR EXISTS (reply r WHERE ViewableComments(r))`, with `visible` computed in a LATERAL.
- Jet makes trivial literals bind parameters (`SELECT $3::integer`, the state strings in the CASE). The output is noisy but harmless; the real query takes 29 args.

### 2.3 EXPLAIN under a real generic plan [measured]

Method: `PREPARE` + `SET plan_cache_mode = force_generic_plan` + `EXPLAIN ANALYZE EXECUTE` (EXPLAIN on a parameterized statement plans with the values, so it would show a custom plan). Latency: 200 runs on a pgx cached statement with the same setting, single client, warm.

| Query | Key plan facts | Exec (EXPLAIN) | p50 / p95 / p99 |
|---|---|---|---|
| Feed, first page, viewer 1 | `Index Scan using posts_feed`, **`Index Cond: (ROW(published_at, id) < ROW($5, $6))`**. Store exposable is a semi join probed per candidate row through the PK; takedowns and blocks are anti joins | 1.4 ms | 1.26 / 3.6 / 7.7 ms |
| Feed, page at depth 40k, viewer 1 | Same plan | 0.7 ms | 1.2 / 8.7 / 19.7 ms |
| Feed, page at depth 40k, anonymous | Same, no blocks join | 0.4 ms | 1.0 / 2.2 / 2.8 ms |
| **Feed with `'PUBLISHED'` as a bind parameter** (`String("PUBLISHED")`) | **Parallel Seq Scan on posts + Sort**: a `$1` cannot prove the partial index `WHERE status='PUBLISHED'` | 59 ms | 14.7 / 25.4 / 40.9 ms |
| Top-level comments, hot post (20k), first page | **`InitPlan` + `One-Time Filter`** for the container (once, not per row). `Index Scan using comments_top_level`, **`Index Cond: (post_id = $13) AND (ROW(created_at, id) > ROW($27, $28))`**. No sort | 22.7 ms (cold) | 2.6 / 4.9 / 8.0 ms |
| Top-level comments, hot post, depth 12k | Same | 4.6 ms | 2.6 / 4.1 / 4.7 ms |
| Trending hashtags, 7 days | `Index Scan using posts_feed`, `Index Cond: published_at >= $3 AND < $4`. Exposable and takedowns are hashed; group plus top-N | 26 ms | 15.1 / 24.1 / 42.2 ms |

Limits of these numbers: local container, warm cache, synthetic data, one client.
- They match the prior report's SRF numbers (feed p95 2.5 ms, top-level p95 5.8 ms).
- The plans are the same shape, because the SQL the planner sees is the same. Only the place that composes it moves from the DB to Go [inferred].
- **New rule this exposed:** fixed codes in predicates must render as inline constants (`NewEnumValue`), never as bind parameters. A unit test (`TestReadablePostsShape`) pins this.

Minor:
- The planner flattened the LATERAL, so `visible` is evaluated in both the filter and the projection. That is about 50 extra index probes per page, which is cheap.
- With a viewer, the two block subqueries become hashed SubPlans that build a ~200-row hash once.

### 2.4 Type safety of result scanning [measured]

| Path | Rename or typo in a column or field | Zero rows |
|---|---|---|
| sqlc (status quo) | Fails at generation | Same |
| jet builder expressions | **Compile error** after regeneration (`s.LikeCount undefined`) | n/a |
| jet QRM, default settings | **Silent.** Without an `alias:"p.*"` tag the result is `rows=0 err=nil`; a misnamed field stays zero | Silent |
| jet QRM, `StrictScan` + `StrictFieldMapping` | Panics on the first row ("columns never used: …" / "fields never mapped") | Silent (the check runs only on row 1) |
| pgx `RowToStructByName` on `.Sql()` (the repo's `db.Conn(ctx)` path) | Error on the first row ("struct doesn't have corresponding row field post_count", "cannot scan text into *int64") | **Silent** |
| **Describe-based shape test** (`shapecheck/`) | Asks Postgres to Describe each registered statement (`PgConn().Prepare`, no execution, no data) and compares column names and OIDs with the row struct. **Caught the typo with zero rows** | Caught |
| bob SQL-file queries | Generation fails (prepare-based); rows are scanned by index into generated structs | Same |

Conclusion:
- A builder gives up sqlc's compile-time check on result columns.
- A Describe test run in `go test` against the migrated test DB brings back the same guarantee at test time. That costs about 60 lines, plus a registry of query constructors.
- Builder expressions themselves are compile-checked, which is stronger than sqlc's `.sql` text.

### 2.5 Enforcing the pub_*-only rule (rule 2) [measured]

sqlc enforces rule 2 because it reads only its own `schema.sql` plus allowed `pub.sql` files. A jet builder generated for schema `social` cannot name seller tables either, except through raw escapes. Bob's SQL-file generator **accepted** `JOIN seller.store_requirement_flags` and produced a typed row, because it types queries against the live DB [measured]. So enforcement has to be explicit:

1. **`sqlboundary`**: a `pg_query_go` parse of SQL text. Every `RangeVar` (a table reference) must be in the module's own schema. Every schema-qualified function outside it must be `<allowed owner>.pub_*`, using the same dependency table as today.
   - Run it over every registered builder statement in a plain unit test. It needs no DB.
   - Optionally also run it over every SQL a pgx `QueryTracer` sees in integration tests. That catches unregistered queries [inferred, not built].
   - Mutation checks: an injected `seller.store_requirement_flags` and `seller.effective_measures(now())` were both reported.
   - The check works for sqlc, jet and bob output alike.
2. **AST test**: `CustomExpression`, `Token` and `Raw*` may appear only in `pub/` wrappers (and in the counted generator). Passed on `visjet` and `queriesjet`.
3. **Registry completeness**: an AST test that every exported constructor in the public-queries package is in the registry [inferred, not built]. A stronger alternative is the ent/Lemmy single entry point: public queries get the posts table only from `vis.PublicPosts(at, viewer)`, which carries the predicate, and an AST test forbids raw `table.Posts` in that package [inferred]. This replaces `readpolicy_test.go`'s condition counting.

### 2.6 Keeping one "counted" definition across the DB trigger and the Go read [measured]

- `cmd/gencounted` renders `visjet.CountedComments(c, NOW())` into a plpgsql `social.comment_counted_now(comment_id)` (`out/counted_generated.sql`). It was created and run on the DB.
- In the repo, the generator would emit the trigger function bodies (plpgsql already carries the rule-6 exception for triggers and binds at run time, like today's `pub_inactive_buyers` use). A golden test would fail on drift.
- The same Go predicate rendered inline is what reads and the daily recount use. Both counted the hot post at **26 929 = 26 929**.
- Reads must **not** call the DB function per row. For 27k rows it took **8 676 ms vs 53 ms** inline, about 320 µs per plpgsql call: it is not inlinable, and each call re-runs its subqueries.
- Answer to the brief's question: the Go predicate is the single definition, and the DB gets a generated copy for single-row trigger and reevaluate use. Viewer rules live only in Go, and they never feed counts.
- The opposite direction (the DB function as the definition, read-side "counted" calling it) is measured as about 160× slower on aggregates.

Unchanged from the prior report: a time-ranged takedown changes membership without any write, so counts need a job at `starts_at`/`ends_at` (or takedowns stay out of "counted"). That decision belongs to Will and does not depend on the builder.

### 2.7 Unit-testability [measured]

- `sqlboundary/boundary_test.go` checks the predicate shape with no DB, from the rendered SQL: anonymous reads never touch blocks, viewer reads do, and status is an inline constant.
- Behaviour against fact combinations (deleted × hidden × inactive × takedown × blocks) still belongs in a DB-backed test. The difference is that it now exercises one Go function instead of 24 queries [inferred].

### 2.8 bob as the second candidate [measured]

- Pure builder: SQL equivalent to jet's, and rows returned through the pgx driver.
- Generated table names render unqualified (`FROM posts`) unless `shared_schema` is set to something other than `social`. Unqualified names fail here (`relation "posts" does not exist`) and break the repo's "always fully qualified" rule.
- **SQL file + appended predicate**: `psql.Select(bobq.FeedBase(...), sm.Where(ReadablePosts))` works, and its rows are typed by index. The documented shortcut `.With(...).All()` **drops the base query's arguments** in v0.50.0 (`WHERE  AND …`, `LIMIT` with no value, a syntax error). That is a library bug.
- Downsides against jet:
  - untyped column expressions;
  - a prepare-based generator that does not enforce the schema boundary;
  - smaller adoption;
  - the bug above.
- Upside: SQL stays in `.sql` files, and only the predicate is in Go. That is the most legible option for people who want to read SQL.

## 3. Constraint ledger

| Constraint | REAL / SELF-IMPOSED | Source | Effect |
|---|---|---|---|
| sqlc as the only query layer | SELF-IMPOSED | backend/AGENTS.md "單體的結構" (`sqlc 生成到 internal/store`); ADR `2026-09-27-module-internal-structure.md` lines 34 and 77. Line 82 lists "要換掉 sqlc" (replacing sqlc) as a re-evaluation trigger. I found no ADR that weighs sqlc against builders: I grepped `docs/adr` for "sqlc" and read only the hits in the monolith and module-structure ADRs, so this negative is [inferred]. sqlc appears to have been carried over from company-era services (`services/*/db/sqlc.yaml`) | This is what forces the rule into every query or into DB objects. Keep sqlc for static queries; exempt the predicate-bearing reads |
| sqlc has no fragments; no `*` or embed on an SRF alias | REAL (tool limit) | sqlc #2244 open [docs]; prior report [measured] | Pushes composition out of sqlc |
| Rule 2: only own schema plus `<owner>.pub_*` | SELF-IMPOSED rule; its **enforcement through sqlc's schema list** is a mechanism we own | AGENTS.md rule 2; ADR monolith lines 326–330 | The goal is kept. Enforcement moves to `sqlboundary` (pg_query) over builder SQL [measured working] |
| Rule 6: functions `BEGIN ATOMIC`, triggers excepted; no whole-row returns | SELF-IMPOSED | AGENTS.md rule 6; `TestSchemaFileMatchesMigrations` | Not touched if the generator emits trigger bodies (plpgsql). The views `published_posts` and `visible_*` get deleted |
| Module migration must apply alone to an empty DB | SELF-IMPOSED | `cmd/hawkr/main_integration_test.go:40` `TestEveryModuleStaysInsideItsOwnSchema` | Not touched: no new DB objects reference other modules (the trigger functions are already plpgsql) |
| Postgres binds views and `BEGIN ATOMIC` bodies at creation | REAL | PG behaviour, prior report [measured] | Moot under a builder |
| A generic plan cannot use a partial index whose predicate compares against a bind parameter | REAL (PG planner) | [measured] 59 ms vs 0.7 ms | Codes are inline constants; pinned by a test |
| Planner rules: container once on the parent key; split block NOT EXISTS; SRF must return the row | REAL | prior report [measured]; here [measured] | The builder follows the first two by construction; the third is moot |
| jet QRM needs `database/sql` | REAL (library) | `qrm/db.go` [read-source] | Do not use QRM: `.Sql()` + `db.Conn(ctx)` + pgx `RowToStructByName` + Describe test |
| jet and bob generators need a live, migrated DB | REAL (tools) | [measured] | The generate step needs Postgres (the repo already runs sqlc in docker and testcontainers in tests) |
| Perf target p95 ≤ 300 ms @ 50 on 10^5 posts | SELF-IMPOSED target | `docs/plans/2026-10-18-a7-social/step-1a-design.md:243`, `d3-read-model-brief.md:326` | Not binding: measured p95 2–25 ms |
| Owner wants logic visible to humans and AI | SELF-IMPOSED preference (Will) | brief | Favors Go predicates plus golden SQL snapshots over DB objects |

## 4. Recommendation

**End state for social:**
1. **Hybrid query layer.** The ~76 static queries (writes, `FOR UPDATE` locks, purge, media refs, owner-only reads) stay in sqlc. The 24 rule-bearing queries (public reads, placeholder lists, hashtag suggest and trending, the 3 recount queries) are built with **go-jet** on top of one `internal/visibility` package (`ReadablePosts`, `CountedComments`, `ViewableComments`, plus the forum equivalents). They run on `db.Conn(ctx)` and scan with pgx `RowToStructByName`.
2. **pub wrappers are generated from each owner's `pub.sql`.** That is the only place raw SQL is allowed (AST test).
3. **Guards that replace what sqlc gave us:**
   - `sqlboundary` (pg_query) over the registry, which keeps rule 2;
   - a Describe shape test for every registered query, which gives result typing with no data needed;
   - golden `.sql` snapshots of every rendered query, so a human or an AI reviewer sees the SQL diff and EXPLAIN tests have stable text [inferred];
   - a single-entry-point type for public tables, which replaces `readpolicy_test.go`'s counting.
4. **"Counted" is defined once, in Go.** A generator emits the plpgsql trigger and reevaluate bodies with a golden drift test. Recount and reads render the same predicate inline. Viewer rules (blocks) live only in Go and never touch counts.
5. **Delete** the views `published_posts`, `visible_comments`, `visible_forum_entries`, `comment_is_hidden`, `comment_counted`, and the prior report's planned SRFs.

Why jet over bob:
- jet: typed operands, compile errors on schema change, larger adoption, and a generator that only sees the schemas you give it.
- bob: untyped columns, a generator that does not enforce the boundary, and a measured argument-dropping bug in `.With().All()`.
- Pick bob only if keeping query text in `.sql` files outweighs those points.

**Repo-wide rule.** sqlc stays the default. A module whose reads share a visibility or eligibility predicate across several queries (social now; A6 review and im are likely candidates, since both get blocks and hidden content) builds those reads with jet on top of its own `internal/visibility` package, under the same guards. This is the "scope" or "finder" pattern from Rails, Django, Lemmy, Gitea and Mattermost, with sqlc kept where nothing needs composing.

**Migration cost** [inferred; not measured]:
- Rewrite the 24 queries: about 1.5 sessions.
- New tooling (jet make target against migrated docker PG, pub-wrapper generator, `sqlboundary`, shape test, golden snapshots, trigger-body generator): about 1–1.5 sessions.
- Arch tests: the `sqlc_test.go` schema-list check stays for sqlc. Add the builder boundary, escape and registry tests. Delete `readpolicy_test.go`.
- Docs: AGENTS rules 2 and 6 wording, plus a revision to ADR `2026-09-27-module-internal-structure` (its re-evaluation condition "換掉 sqlc", replacing sqlc, is partly met) and the monolith ADR's enforcement section.
- The prior report's alternative (DB SRFs + `reevaluate`) also changes 24 queries and adds a smoke test for string-body functions, so the two cost about the same. The builder route avoids the rule-6 and "applied alone" exceptions.

**Triggers to change approach:**
- jet stalls (no release in 12 months) or breaks with pgx or PG versions → switch the same predicate package to bob's builder (the predicate API shape carries over).
- A measured public read p95 > 300 ms, or `reevaluate` fan-out blows the job budget → move to a projected read model through outbox. Predicates then run in the projector.
- Modules move to separate databases → cross-schema SQL is gone and the `pub_*` joins become API calls or projections.
- sqlc ships fragments or macros (#2244) → re-evaluate folding the builder queries back into sqlc.

## 5. Notes

- **The Docker Desktop VM disk is full: 55.4 of 58.4 GB used, 4–110 MB free.** My first seed on a disk-backed volume failed with ENOSPC. I reran on tmpfs and touched nothing else. Other sessions' containers (testcontainers, `a7-1a-perf2`) are probably at risk. Build cache (16 GB reclaimable) and dangling volumes (14 GB) are the obvious targets; I did not prune because that deletes data.
- Container `builder-spike-pg` (port 55901, tmpfs, about 850 MB RAM) is left running for verification. `docker --context desktop-linux stop builder-spike-pg` removes it.
