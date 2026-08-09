import { pathToFileURL } from "node:url";
import ccxt from "../ccxt/js/ccxt.js";

export function classifyError(error) {
  const name = String(error?.name ?? "");
  const message = String(error?.message ?? "").toLowerCase();
  if (message.includes("ip") && (message.includes("whitelist") || message.includes("restricted"))) return "IP_RESTRICTED";
  if (name === "InvalidNonce" || message.includes("timestamp") || message.includes("recvwindow")) return "CLOCK_SKEW";
  if (name === "AuthenticationError") return "AUTHENTICATION_FAILED";
  if (name === "PermissionDenied") return "PERMISSION_DENIED";
  if (name.includes("Network") || name === "RequestTimeout") return "NETWORK_ERROR";
  return "EXCHANGE_ERROR";
}

export function validateEnvironment(environment = process.env) {
  const required = ["BINANCE_API_KEY", "BINANCE_API_SECRET", "GATEIO_API_KEY", "GATEIO_API_SECRET"];
  return required.filter((name) => !environment[name]);
}

export function createExchanges(environment = process.env) {
  return [
    ["binance", new ccxt.binance({
      apiKey: environment.BINANCE_API_KEY,
      secret: environment.BINANCE_API_SECRET,
      enableRateLimit: true,
      options: {
        defaultType: "spot",
        fetchCurrencies: false,
        fetchMargins: false,
        fetchMarkets: { types: ["spot"] },
      },
    })],
    ["gateio", new ccxt.gate({
      apiKey: environment.GATEIO_API_KEY,
      secret: environment.GATEIO_API_SECRET,
      enableRateLimit: true,
      options: {
        defaultType: "spot",
        unifiedAccount: false,
        fetchMarkets: { types: ["spot"] },
      },
    })],
  ];
}

export async function checkExchange(name, exchange) {
  let result = false;
  try {
    await exchange.loadMarkets();
    console.log(`[${name}] public: PASS`);

    try {
      await exchange.fetchBalance({ type: "spot" });
      console.log(`[${name}] private-read: PASS`);
      result = true;
    } catch (error) {
      console.log(`[${name}] private-read: FAIL (${classifyError(error)})`);
    }
  } catch (error) {
    console.log(`[${name}] public: FAIL (${classifyError(error)})`);
  } finally {
    try {
      await exchange.close?.();
    } catch (error) {
      console.log(`[${name}] cleanup: FAIL (${classifyError(error)})`);
      result = false;
    }
  }
  return result;
}

async function main() {
  const missing = validateEnvironment();
  if (missing.length > 0) {
    console.error(`Missing required variables: ${missing.join(", ")}`);
    process.exitCode = 1;
    return;
  }

  const exchanges = createExchanges();
  const results = [];
  for (const [name, exchange] of exchanges) results.push(await checkExchange(name, exchange));
  if (results.some((result) => !result)) process.exitCode = 1;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    await main();
  } catch (error) {
    console.error(`[verifier] FAIL (${classifyError(error)})`);
    process.exitCode = 1;
  }
}
