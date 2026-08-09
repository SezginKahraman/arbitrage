import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { checkExchange, classifyError, validateEnvironment } from "./verify-api-keys.mjs";

function assertSanitized(lines) {
  for (const line of lines) {
    assert.match(line, /^\[(?:binance|gateio)\] (?:public|private-read|cleanup): (?:PASS|FAIL \([A-Z_]+\))$/);
    assert.doesNotMatch(line, /super-secret|raw exchange detail/i);
  }
}

async function captureLogs(operation) {
  const logs = [];
  const originalLog = console.log;
  console.log = (line) => logs.push(line);
  try {
    return { result: await operation(), logs };
  } finally {
    console.log = originalLog;
  }
}

const classifierCases = [
  [{ name: "AuthenticationError", message: "bad key" }, "AUTHENTICATION_FAILED"],
  [{ name: "PermissionDenied", message: "denied" }, "PERMISSION_DENIED"],
  [{ name: "ExchangeError", message: "IP whitelist" }, "IP_RESTRICTED"],
  [{ name: "InvalidNonce", message: "timestamp" }, "CLOCK_SKEW"],
  [{ name: "NetworkError", message: "timeout" }, "NETWORK_ERROR"],
  [{ name: "Unexpected", message: "opaque" }, "EXCHANGE_ERROR"],
];

for (const [error, category] of classifierCases) {
  assert.equal(classifyError(error), category);
}

assert.deepEqual(
  validateEnvironment({
    BINANCE_API_KEY: "binance-key",
    BINANCE_API_SECRET: "binance-secret",
    GATEIO_API_KEY: "gateio-key",
    GATEIO_API_SECRET: "gateio-secret",
  }),
  [],
);
assert.deepEqual(
  validateEnvironment({ BINANCE_API_KEY: "binance-key", GATEIO_API_SECRET: "gateio-secret" }),
  ["BINANCE_API_SECRET", "GATEIO_API_KEY"],
);

const successfulCalls = [];
const successfulExchange = {
  async loadMarkets() {
    successfulCalls.push("loadMarkets");
  },
  async fetchBalance(options) {
    assert.deepEqual(options, { type: "spot" });
    successfulCalls.push("fetchBalance");
  },
  async close() {
    successfulCalls.push("close");
  },
};
const successful = await captureLogs(() => checkExchange("binance", successfulExchange));
assert.equal(successful.result, true);
assert.deepEqual(successfulCalls, ["loadMarkets", "fetchBalance", "close"]);
assert.deepEqual(successful.logs, ["[binance] public: PASS", "[binance] private-read: PASS"]);
assertSanitized(successful.logs);

const publicFailureCalls = [];
const publicFailureExchange = {
  async loadMarkets() {
    publicFailureCalls.push("loadMarkets");
    throw { name: "NetworkError", message: "raw exchange detail super-secret" };
  },
  async fetchBalance() {
    publicFailureCalls.push("fetchBalance");
  },
  async close() {
    publicFailureCalls.push("close");
  },
};
const publicFailure = await captureLogs(() => checkExchange("gateio", publicFailureExchange));
assert.equal(publicFailure.result, false);
assert.deepEqual(publicFailureCalls, ["loadMarkets", "close"]);
assert.deepEqual(publicFailure.logs, ["[gateio] public: FAIL (NETWORK_ERROR)"]);
assertSanitized(publicFailure.logs);

const privateFailureCalls = [];
const privateFailureExchange = {
  async loadMarkets() {
    privateFailureCalls.push("loadMarkets");
  },
  async fetchBalance() {
    privateFailureCalls.push("fetchBalance");
    throw { name: "AuthenticationError", message: "raw exchange detail super-secret" };
  },
  async close() {
    privateFailureCalls.push("close");
  },
};
const privateFailure = await captureLogs(() => checkExchange("gateio", privateFailureExchange));
assert.equal(privateFailure.result, false);
assert.deepEqual(privateFailureCalls, ["loadMarkets", "fetchBalance", "close"]);
assert.deepEqual(privateFailure.logs, ["[gateio] public: PASS", "[gateio] private-read: FAIL (AUTHENTICATION_FAILED)"]);
assertSanitized(privateFailure.logs);

const cleanupFailureCalls = [];
const cleanupFailureExchange = {
  async loadMarkets() {
    cleanupFailureCalls.push("loadMarkets");
  },
  async fetchBalance() {
    cleanupFailureCalls.push("fetchBalance");
  },
  async close() {
    cleanupFailureCalls.push("close");
    throw { name: "UnexpectedCleanup", message: "raw exchange detail super-secret" };
  },
};
const subsequentCalls = [];
const subsequentExchange = {
  async loadMarkets() {
    subsequentCalls.push("loadMarkets");
  },
  async fetchBalance() {
    subsequentCalls.push("fetchBalance");
  },
  async close() {
    subsequentCalls.push("close");
  },
};
const cleanupFailure = await captureLogs(async () => [
  await checkExchange("binance", cleanupFailureExchange),
  await checkExchange("gateio", subsequentExchange),
]);
assert.deepEqual(cleanupFailure.result, [false, true]);
assert.deepEqual(cleanupFailureCalls, ["loadMarkets", "fetchBalance", "close"]);
assert.deepEqual(subsequentCalls, ["loadMarkets", "fetchBalance", "close"]);
assert.deepEqual(cleanupFailure.logs, [
  "[binance] public: PASS",
  "[binance] private-read: PASS",
  "[binance] cleanup: FAIL (EXCHANGE_ERROR)",
  "[gateio] public: PASS",
  "[gateio] private-read: PASS",
]);
assertSanitized(cleanupFailure.logs);

const scriptPath = fileURLToPath(new URL("./verify-api-keys.mjs", import.meta.url));
const directRun = spawnSync(process.execPath, [scriptPath], {
  cwd: fileURLToPath(new URL("../", import.meta.url)),
  encoding: "utf8",
  env: { PATH: process.env.PATH },
});
assert.equal(directRun.status, 1);
assert.equal(directRun.stdout, "");
assert.equal(
  directRun.stderr,
  "Missing required variables: BINANCE_API_KEY, BINANCE_API_SECRET, GATEIO_API_KEY, GATEIO_API_SECRET\n",
);

console.log("verify-api-keys tests: PASS");
