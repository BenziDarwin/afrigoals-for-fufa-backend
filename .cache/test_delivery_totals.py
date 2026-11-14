from pathlib import Path
import json
api = Path('F:/Afrigoals/Afrigoals_ecommerce_api')
web = Path('F:/Afrigoals/Afrigoals_ecommerce')
def put(root, name, text):
    path = root / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding='utf-8')
put(api, 'services/delivery_settings_test.go', '''package services

import (
    "context"
    "math"
    "testing"
)

func TestConfiguredFlatDeliveryUses300WithoutGoogleRequest(t *testing.T) {
    t.Setenv("DELIVERY_PRICING_MODE", "flat")
    t.Setenv("DELIVERY_FLAT_FEE", "300")
    q, err := ConfiguredDeliveryQuoter(nil, "")
    if err != nil { t.Fatal(err) }
    quote, err := q.Quote(context.Background(), .3, 32.5)
    if err != nil || quote.Fee != 300 { t.Fatalf("quote = %#v, %v", quote, err) }
    if 500 + quote.Fee != 800 { t.Fatal("checkout and payment must both total 800") }
}

func TestDeliverySettingsValidationAndModeSwitch(t *testing.T) {
    settings := DefaultDeliverySettings()
    for _, mode := range []string{"flat", "distance"} {
        settings.Mode = mode
        if err := ValidateDeliverySettings(settings); err != nil { t.Fatal(err) }
    }
    for _, mutate := range []func(*DeliverySettings){
        func(s *DeliverySettings) { s.Mode = "invalid" },
        func(s *DeliverySettings) { s.FlatFee = -1 },
        func(s *DeliverySettings) { s.Pricing.MotorcycleKmPerLitre = 0 },
        func(s *DeliverySettings) { s.Pricing.RoundToNearest = 0 },
        func(s *DeliverySettings) { s.Pricing.PeakMultiplier = math.NaN() },
        func(s *DeliverySettings) { s.Pricing.OriginLat = 100 },
    } {
        invalid := settings
        mutate(&invalid)
        if ValidateDeliverySettings(invalid) == nil { t.Fatal("invalid settings accepted") }
    }
}

func TestExpectedTotalRejectsSilentPaymentAmountChange(t *testing.T) {
    expected := 800.0
    if CheckExpectedOrderTotal(&expected, 800) != nil { t.Fatal("matching total rejected") }
    if CheckExpectedOrderTotal(&expected, 6000) == nil { t.Fatal("checkout 800 must not charge 6000") }
    expected = math.NaN()
    if CheckExpectedOrderTotal(&expected, 800) == nil { t.Fatal("NaN expected total accepted") }
}
''')
put(api, 'routes/delivery_pricing_test.go', '''package routes

import (
    "encoding/json"
    "net/http/httptest"
    "strings"
    "testing"
    "github.com/gofiber/fiber/v2"
)

func TestDeliveryQuoteUsesTheConfiguredOrderPricing(t *testing.T) {
    t.Setenv("DELIVERY_PRICING_MODE", "flat")
    t.Setenv("DELIVERY_FLAT_FEE", "300")
    app := fiber.New()
    RegisterDeliveryPricing(app, func(c *fiber.Ctx) error { return c.Next() }, Deps{})
    for _, tt := range []struct { body string; status int }{
        {`{"lat":0.3,"lng":32.5}`, 200},
        {`{"lat":100,"lng":32.5}`, 400},
        {`{}`, 400},
    } {
        req := httptest.NewRequest("POST", "/delivery/quote", strings.NewReader(tt.body))
        req.Header.Set("Content-Type", "application/json")
        res, err := app.Test(req)
        if err != nil { t.Fatal(err) }
        if res.StatusCode != tt.status { t.Fatalf("status = %d; want %d", res.StatusCode, tt.status) }
        if tt.status == 200 {
            var quote map[string]any
            if err := json.NewDecoder(res.Body).Decode(&quote); err != nil { t.Fatal(err) }
            if quote["fee"] != float64(300) { t.Fatalf("quote = %#v", quote) }
        }
        res.Body.Close()
    }
}

func TestCustomersCannotChangeDeliveryPricing(t *testing.T) {
    app := fiber.New()
    RegisterDeliveryPricing(app, func(c *fiber.Ctx) error {
        c.Locals("role", "customer")
        return c.Next()
    }, Deps{})
    for _, method := range []string{"GET", "PUT"} {
        res, err := app.Test(httptest.NewRequest(method, "/admin/delivery-pricing", strings.NewReader(`{}`)))
        if err != nil { t.Fatal(err) }
        res.Body.Close()
        if res.StatusCode != 403 { t.Fatalf("customer %s returned %d", method, res.StatusCode) }
    }
}
''')
put(web, 'tests/delivery-pricing.test.ts', '''import "./setup-server-only";
import assert from "node:assert/strict";
import { afterEach, mock, test } from "node:test";
import { getDeliveryQuote } from "../lib/actions/delivery";
import { createCashOnDeliveryOrder } from "../lib/actions/create-cash-on-delivery-order";
import { AUTH_COOKIE_NAME } from "../lib/auth/constants";
import { GET, PUT } from "../app/api/admin/delivery-pricing/route";
import { mockFetch, json, setRequestCookies } from "./helpers";

process.env.AFRIGOALS_API_URL = "https://api.afrigoals-store.com";
afterEach(() => mock.restoreAll());

test("checkout displays the API delivery fee rather than local pricing settings", async () => {
  const calls = mockFetch(() => json({ fee: 300, distanceKm: 0, distanceMeters: 0, currency: "UGX" }));
  const quote = await getDeliveryQuote({ lat: .3, lng: 32.5 });
  assert.equal(quote.fee, 300);
  assert.equal(quote.success, true);
  assert.equal(calls[0].url, "https://api.afrigoals-store.com/api/v1/delivery/quote");
});
test("changed admin rates are reflected by the next checkout quote", async () => {
  let fee = 300;
  mockFetch(() => json({ fee, distanceKm: 4, currency: "UGX" }));
  assert.equal((await getDeliveryQuote({ lat: .3, lng: 32.5 })).fee, 300);
  fee = 5500;
  assert.equal((await getDeliveryQuote({ lat: .3, lng: 32.5 })).fee, 5500);
});
test("a total conflict never sends an IOTEC collection request", async () => {
  setRequestCookies({ [AUTH_COOKIE_NAME]: "customer-token" });
  const calls = mockFetch(() => json({ error: "Your total has changed" }, 409));
  const result = await createCashOnDeliveryOrder(
    [{ productId: "p1", name: "Kit", price: 500, quantity: 1 }],
    { address: "Kampala", lat: .3, lng: 32.5, fee: 300, distanceKm: 0, contactName: "Customer", contactPhone: "+256771234567" },
    "mobile_money", "+256771234567", 800,
  );
  assert.equal(result.success, false);
  assert.equal(calls.length, 1);
  assert.equal(JSON.parse(String(calls[0].init?.body)).expectedTotal, 800);
});
test("admin pricing proxy rejects guests without calling the API", async () => {
  setRequestCookies({});
  const calls = mockFetch(() => json({}));
  assert.equal((await GET()).status, 401);
  assert.equal((await PUT(new Request("http://localhost/api/admin/delivery-pricing", { method: "PUT", body: "{}" }))).status, 401);
  assert.equal(calls.length, 0);
});
''')
path = web / 'package.json'
package = json.loads(path.read_text(encoding='utf-8'))
package['scripts']['test'] += ' tests/delivery-pricing.test.ts'
put(web, 'package.json', json.dumps(package, indent=2)+'\n')
print('Added delivery quote, pricing authorization and payment-total regression tests.')
