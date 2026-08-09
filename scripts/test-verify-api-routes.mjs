import assert from "node:assert/strict";
import test from "node:test";
import ccxt from "../ccxt/js/ccxt.js";
import { checkExchange, createExchanges } from "./verify-api-keys.mjs";

const OFFLINE_ENVIRONMENT = {
  BINANCE_API_KEY: "offline-binance-key",
  BINANCE_API_SECRET: "offline-binance-secret",
  GATEIO_API_KEY: "offline-gate-key",
  GATEIO_API_SECRET: "offline-gate-secret",
};

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

function productionClient(label) {
  const clients = new Map(createExchanges(OFFLINE_ENVIRONMENT));
  const client = clients.get(label);
  assert.ok(client, `missing production client for ${label}`);
  return client;
}

function interceptImplicitRoutes(exchange, allowedRoutes, forbiddenRoutes) {
  const calls = [];

  for (const method of forbiddenRoutes) {
    assert.equal(typeof exchange[method], "function", `${exchange.id}.${method} must exist in CCXT ${ccxt.version}`);
    exchange[method] = async () => {
      calls.push(method);
      throw new Error(`FORBIDDEN_READ_ROUTE:${method}`);
    };
  }

  exchange.request = async (path, api, method) => {
    const namespace = Array.isArray(api) ? api.join(".") : api;
    const route = `${method}|${namespace}|${path}`;
    calls.push(route);
    if (!Object.hasOwn(allowedRoutes, route)) throw new Error("UNEXPECTED_NETWORK_ROUTE");
    return allowedRoutes[route];
  };

  return calls;
}

test("Binance production client loads only public spot markets and verifies through the spot account route", async () => {
  assert.equal(ccxt.version, "4.5.71");
  const exchange = productionClient("binance");
  assert.ok(exchange instanceof ccxt.binance);

  const calls = interceptImplicitRoutes(
    exchange,
    {
      "GET|public|exchangeInfo": {
        timezone: "UTC",
        serverTime: 0,
        rateLimits: [],
        exchangeFilters: [],
        symbols: [],
      },
      "GET|private|account": {
        makerCommission: 0,
        takerCommission: 0,
        buyerCommission: 0,
        sellerCommission: 0,
        canTrade: true,
        canWithdraw: false,
        canDeposit: true,
        updateTime: 0,
        accountType: "SPOT",
        balances: [],
        permissions: ["SPOT"],
      },
    },
    [
      "sapiGetCapitalConfigGetall",
      "sapiGetMarginAllPairs",
      "sapiGetMarginIsolatedAllPairs",
      "fapiPublicGetExchangeInfo",
      "dapiPublicGetExchangeInfo",
      "eapiPublicGetExchangeInfo",
      "sapiGetMarginAccount",
      "sapiGetMarginIsolatedAccount",
      "papiGetBalance",
      "fapiPrivateV3GetAccount",
      "fapiPrivateV2GetAccount",
      "dapiPrivateGetAccount",
      "sapiPostAssetGetFundingAsset",
    ],
  );

  const verification = await captureLogs(() => checkExchange("binance", exchange));

  assert.deepEqual(calls, ["GET|public|exchangeInfo", "GET|private|account"]);
  assert.equal(verification.result, true);
  assert.deepEqual(verification.logs, ["[binance] public: PASS", "[binance] private-read: PASS"]);
});

test("Gate production client loads only public spot markets and verifies through spot accounts", async () => {
  assert.equal(ccxt.version, "4.5.71");
  const exchange = productionClient("gateio");
  assert.ok(exchange instanceof ccxt.gate);

  const calls = interceptImplicitRoutes(
    exchange,
    {
      "GET|public.spot|currencies": [],
      "GET|public.margin|currency_pairs": [],
      "GET|public.spot|currency_pairs": [],
      "GET|private.spot|accounts": [],
    },
    [
      "privateAccountGetDetail",
      "privateUnifiedGetAccounts",
      "publicFuturesGetSettleContracts",
      "publicDeliveryGetSettleContracts",
      "publicOptionsGetUnderlyings",
      "publicOptionsGetContracts",
      "privateMarginGetAccounts",
      "privateMarginGetCrossAccounts",
      "privateMarginGetFundingAccounts",
      "privateFuturesGetSettleAccounts",
      "privateDeliveryGetSettleAccounts",
      "privateOptionsGetAccounts",
    ],
  );

  const verification = await captureLogs(() => checkExchange("gateio", exchange));

  assert.deepEqual(calls, [
    "GET|public.spot|currencies",
    "GET|public.margin|currency_pairs",
    "GET|public.spot|currency_pairs",
    "GET|private.spot|accounts",
  ]);
  assert.equal(verification.result, true);
  assert.deepEqual(verification.logs, ["[gateio] public: PASS", "[gateio] private-read: PASS"]);
});
