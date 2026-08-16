# Lessons

- Binance `-2015` is not enough to blame the credential itself: VPN/IP-whitelist
  changes can make the same signed read-only calls pass and then fail as the
  outbound address changes. Re-test server time and the exact signed endpoint
  after the network path changes, and make metadata collection degrade per
  venue without stopping public market feeds.
- Transfer-network availability is directional and dynamic. Evaluate withdrawal
  on the buy venue and deposit on the sell venue, retain metadata from the side
  that is still available, and distinguish an exact contract/raw-network match
  from a normalized alias that still needs a human check.
- A top-level `loadMarkets()` or `fetchBalance({ type: "spot" })` assertion does not prove an exchange endpoint boundary. For credentialed CCXT clients, test the real version-pinned client with offline implicit-route interception, fail closed on every unexpected transport call, and assert the exact public/private route sequence before running live checks.
- Treat historical verification evidence as superseded when later review disproves its stated scope. Preserve the original result and chronology, state the additional read-only calls transparently, and collect fresh evidence only after the route regression tests pass.
- Origin URLs, shallow HEADs, and mutable Docker tags are not interchangeable reproducibility evidence. Record each resolved nested Git HEAD and the running image's repository digest, and distinguish that observed snapshot from a version or image pin.
- When an exchange reports that an API key does not exist even though the user
  has verified the value, test the exact site-specific domains with the same
  read-only route before blaming the credential. KuCoin Global and KuCoin EU
  use separate key namespaces: a Global key passed on `api.kucoin.com` while
  the same key returned `400003` on `api.kucoin.eu`.
- Adding a low-priced market requires consumer-visible precision verification,
  not only checking the application's string formatter. Chart libraries can
  retain their own two-decimal default; test the rendered series price format
  and keep chart labels, source prices, and opportunity prices consistent.
- For browser-visible static asset changes, verifying source and formatter
  tests is insufficient when the HTTP path has no cache policy. Version the
  subresource URLs, set an explicit cache policy for development, and verify
  the headers from the running server after restart.
- The user explicitly does not want separate specs or approval gates for small,
  obvious changes. For those tasks, keep the plan concise in `tasks/todo.md`,
  apply TDD where code behavior changes, and proceed directly to implementation.
- In zsh, lowercase `path` is tied to the shell's executable `PATH`; never use
  it as a loop or task variable. Use a scoped name such as `repo_file` and run
  multi-assertion verification scripts with fail-fast enabled so a missing
  command cannot masquerade as a passing negative check.
- In zsh, `status` is also a read-only special parameter. Use task-specific
  names such as `health_code` or `response_code` in verification loops.
- Docker Desktop's CLI and `docker-credential-desktop` must be discoverable in
  the same execution environment. A login shell can expose `docker` while
  omitting its credential helper; diagnose both paths first, then use the exact
  Docker Desktop resource directory for builds instead of changing project
  configuration.
- After flattening nested repositories into a monorepo, re-audit every `.env`
  path against the outer index. A nested repository's ignore behavior does not
  undo files already tracked by the wrapper; remove them from the outer index
  while preserving the user's local file.
- A scanner health signal must describe executable market coverage, not generic
  data activity. Reference-price ticks and a single order book can be fresh
  while no cross-source route is computable; require two distinct, valid, fresh
  books for the same symbol.
- A non-blocking persistence queue must preserve the semantic first, peak, and
  latest observations when coalescing. On shutdown, cancel any in-flight batch,
  requeue unfinished routes, drain everything under one overall deadline, and
  always attempt to close the store.
- High-frequency chart state needs both bounded retention and bounded update
  frequency. Coarse time buckets plus stable state references prevent React
  effects from rebuilding entire multi-hour series on every order-book tick.
- A live opportunity stream needs authoritative route snapshots in addition to
  throttled deltas. Snapshot the current route set for new clients and publish
  an empty replacement when a route set closes so stale opportunities cannot
  linger as client-local state.
- Expanding from one best route to every qualifying route multiplies storage
  pressure. Coalesce observations over a short time window and persist the
  entire batch in one SQLite transaction while retaining each route's first,
  peak, and latest points.
- SQLite WAL maintenance must validate the result row, not just the SQL call.
  `wal_checkpoint` can return `busy=1` without a SQL error; scan all three result
  columns, fail explicitly when blocked, and cap the journal so an old high-water
  mark cannot turn every restart into a multi-gigabyte replay.
- Never let a successful diagnosis sound like a deployed fix. When live evidence
  proves a bug but the turn only authorizes diagnosis, say explicitly that the
  running code is still unchanged; on the follow-up fix, verify the exact user-
  visible metric rather than only the lower-level data feed.
- A user-visible log retention promise must be time-based, not a small global
  row count shared by unrelated symbols. High-frequency markets can evict a
  quiet selected pair almost immediately; combine a clear TTL with per-stream
  sampling and a high defensive memory cap.
- Event-driven book-ticker silence is not proof that a book is invalid. Track
  quote validation separately from quote changes and use a rate-limited public
  REST refresh before expiring a quiet but still executable market.
- A healthy backend feed can still look unstable when browser delivery is
  unbounded. Preserve full-rate server-side calculations, but cap/coalesce UI
  quote publications per source and pair and verify the WebSocket stays open
  beyond the client freshness window.
- A desktop sidebar inside a stretching CSS grid must be sized and positioned
  against the viewport, not the page content. Keep persistent navigation
  sticky with an explicit viewport height so bottom actions remain reachable
  on short screens and long pages.
- A management action that exists only under a domain-specific label is still
  undiscoverable. Use the user's object name (for example, "pair" instead of
  "market") and place a short instruction beside the action that explains the
  exact add flow.
- A source toggle is misleading if it filters one page while aggregate pages
  continue ranking that source. Treat venue visibility as one workspace-wide
  preference and apply it before hero selection, counts, tables, charts, and
  filter options.
