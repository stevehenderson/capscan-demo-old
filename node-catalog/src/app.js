const express = require("express");
const fetch = require("node-fetch");

const products = [
  { id: "sku-100", name: "Widget" },
  { id: "sku-200", name: "Gadget" },
];

// createApp builds the Express app. fetchImpl is injectable for tests.
function createApp({ pricingUrl, fetchImpl = fetch }) {
  const app = express();
  app.disable("x-powered-by");

  app.get("/products", (req, res) => {
    res.json(products);
  });

  app.get("/products/:id/price", async (req, res) => {
    const product = products.find((p) => p.id === req.params.id);
    if (!product) {
      return res.status(404).json({ error: "not found" });
    }
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 2000);
    try {
      const upstream = await fetchImpl(
        `${pricingUrl}/prices/${encodeURIComponent(product.id)}`,
        { signal: controller.signal },
      );
      if (!upstream.ok) {
        return res.status(502).json({ error: "pricing unavailable" });
      }
      const { amount, currency } = await upstream.json();
      return res.json({ id: product.id, amount, currency });
    } catch {
      return res.status(502).json({ error: "pricing unavailable" });
    } finally {
      clearTimeout(timer);
    }
  });

  return app;
}

module.exports = { createApp };
