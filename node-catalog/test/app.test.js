const fetch = require("node-fetch");
const { createApp } = require("../src/app");

function listen(app) {
  return new Promise((resolve) => {
    const server = app.listen(0, () => resolve(server));
  });
}

function fakePricing(response) {
  return async () => response;
}

describe("node-catalog", () => {
  let server;
  let base;

  async function start(fetchImpl) {
    server = await listen(createApp({ pricingUrl: "http://pricing.test", fetchImpl }));
    base = `http://127.0.0.1:${server.address().port}`;
  }

  afterEach(() => new Promise((resolve) => server.close(resolve)));

  test("lists products", async () => {
    await start(fakePricing({}));
    const res = await fetch(`${base}/products`);
    expect(res.status).toBe(200);
    expect(await res.json()).toHaveLength(2);
  });

  test("returns a price from the pricing service", async () => {
    await start(fakePricing({ ok: true, json: async () => ({ amount: 999, currency: "USD" }) }));
    const res = await fetch(`${base}/products/sku-100/price`);
    expect(await res.json()).toEqual({ id: "sku-100", amount: 999, currency: "USD" });
  });

  test("maps pricing failures to 502", async () => {
    await start(async () => {
      throw new Error("connection refused");
    });
    const res = await fetch(`${base}/products/sku-100/price`);
    expect(res.status).toBe(502);
  });

  test("returns 404 for unknown products", async () => {
    await start(fakePricing({}));
    const res = await fetch(`${base}/products/sku-999/price`);
    expect(res.status).toBe(404);
  });
});
