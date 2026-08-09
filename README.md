# arbitrage

Crypto arbitrage research workspace containing:

- `crypto-futures-arbitrage-scanner`: public, read-only live scanner UI.
- `hummingbot`: upstream trading framework source.
- `ccxt`: upstream multi-exchange API library source.
- `scripts/verify-api-keys.mjs`: sanitized, read-only Binance Global and
  Gate.io credential verifier.

The live scanner currently supports BTC, ETH, XRP, SOL, and COTI. Start it with
Go 1.24 and open `http://localhost:8082`.

Credentials belong only in the ignored root `.env` file. Never commit API
keys, secrets, or passphrases.
