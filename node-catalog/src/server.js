require("dotenv").config();
const { createApp } = require("./app");

const port = Number(process.env.PORT || 3000);
const app = createApp({ pricingUrl: process.env.PRICING_URL });

app.listen(port, () => {
  console.log(`node-catalog listening on ${port}`);
});
