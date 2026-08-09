# Lessons

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
- The user explicitly does not want separate specs or approval gates for small,
  obvious changes. For those tasks, keep the plan concise in `tasks/todo.md`,
  apply TDD where code behavior changes, and proceed directly to implementation.
