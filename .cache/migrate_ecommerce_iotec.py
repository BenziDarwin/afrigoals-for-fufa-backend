from pathlib import Path
import re

api = Path('F:/Afrigoals/Afrigoals_ecommerce_api')
web = Path('F:/Afrigoals/Afrigoals_ecommerce')

def put(path, text):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding='utf-8')

orders = api / 'routes/orders.go'
text = orders.read_text(encoding='utf-8')
text = text.replace('paymentMethod != "cod" && paymentMethod != "pesapal" && paymentMethod != mobileMoneyMethod', 'paymentMethod != "cod" && paymentMethod != mobileMoneyMethod')
text = text.replace('"error": "invalid paymentMethod"', '"error": "Choose IOTEC mobile money or cash on delivery"')
start = text.index('\trouter.Post("/orders/checkout/pesapal"')
end = text.index('\n\trouter.Get("/orders/my"', start)
text = text[:start] + '\trouter.Post("/orders/checkout/pesapal", auth, retiredPaymentProvider)\n' + text[end:]
start = text.index('\trouter.Get("/orders/payment-callback"')
end = text.index('\n\trouter.', start + 10)
text = text[:start] + '\trouter.Get("/orders/payment-callback", auth, retiredPaymentProvider)\n' + text[end:]
text = text.replace('paymentMethod == "pesapal" || paymentMethod == mobileMoneyMethod', 'paymentMethod == mobileMoneyMethod')
text = text.replace('\tif paymentMethod == "pesapal" {\n\t\teventNote = "Order created, awaiting Pesapal payment"\n\t}\n', '')
start = text.index('\nfunc normalizePesapalPaymentStatus(')
text = text[:start] + '\n'
put(orders, text)

payments = api / 'routes/payments.go'
text = payments.read_text(encoding='utf-8')
helper = text[text.index('func firstNonEmpty('):]
put(payments, '''package routes

import (
    "strings"
    "github.com/gofiber/fiber/v2"
)

// Retired URLs remain explicit so old clients cannot initiate another provider.
func retiredPaymentProvider(c *fiber.Ctx) error {
    return c.Status(fiber.StatusGone).JSON(fiber.Map{
        "error": "Pesapal has been retired. Use IOTEC mobile money or cash on delivery.",
    })
}

func RegisterPayments(router fiber.Router, deps Deps) {
    router.All("/payments/pesapal/ipn", retiredPaymentProvider)
}

''' + helper)

# Keep the old exported action as a compatibility entrypoint for existing callers.
put(web / 'lib/actions/checkout.ts', '''"use server";

import { createCashOnDeliveryOrder } from "@/lib/actions/create-cash-on-delivery-order";

type CartItem = Parameters<typeof createCashOnDeliveryOrder>[0][number];
type DeliveryInput = Omit<Parameters<typeof createCashOnDeliveryOrder>[1], "fee" | "distanceKm">;

export async function createCheckoutSession(
  items: CartItem[],
  delivery: DeliveryInput,
  payerPhone?: string,
) {
  // Go calculates the authoritative delivery fee; no redirect gateway is used.
  return createCashOnDeliveryOrder(
    items,
    { ...delivery, fee: 0, distanceKm: 0 },
    "mobile_money",
    payerPhone ?? delivery.contactPhone,
  );
}
''')

page = web / 'app/(app)/checkout/success/page.tsx'
text = page.read_text(encoding='utf-8').replace('params.paymentMethod ?? "pesapal"', 'params.paymentMethod ?? "mobile_money"')
start = text.index('  if (\n')
end = text.index('  return (', start)
text = text[:start] + '''  if ((paymentMethod !== "cod" && paymentMethod !== "mobile_money") || !orderNumber) {
    redirect("/orders");
  }

''' + text[end:]
put(page, text)
page = web / 'app/(app)/checkout/success/SuccessClient.tsx'
text = page.read_text(encoding='utf-8').replace('  const isMobileMoney = paymentMethod === "mobile_money";\n', '')
text = re.sub(r'\{isMobileMoney\s*\? "IOTEC Mobile Money \(MTN / Airtel\)"\s*: "Pesapal"\}', 'IOTEC Mobile Money (MTN / Airtel)', text)
put(page, text)

# Legacy Next handlers must not call a retired gateway, even if bookmarked.
retired = '''import { NextResponse } from "next/server";

export async function GET() {
  return NextResponse.json(
    { error: "Pesapal has been retired. Use IOTEC mobile money or cash on delivery." },
    { status: 410 },
  );
}

export const POST = GET;
'''
for folder in ('app/api/pesapal', 'app/(app)/pesapal'):
    for path in (web / folder).rglob('*.ts'):
        put(path, retired)
put(web / 'lib/pesapal/auth.ts', '''// Compatibility guard: no credentials or requests for the retired provider.
export async function getPesapalToken(): Promise<never> {
  throw new Error("Pesapal has been retired. Use IOTEC mobile money.");
}
''')
put(web / 'lib/pesapal/verify-payment.ts', '''export async function verifyPesapalTransaction(_orderTrackingId: string): Promise<never> {
  throw new Error("Pesapal has been retired. Use IOTEC payment verification.");
}
''')
put(web / 'scripts/register-pesapal-ipn.ts', '''throw new Error("Pesapal has been retired. Configure the Go API IOTEC callback instead.");
export {};
''')

env = web / '.env.example'
text = env.read_text(encoding='utf-8')
start = text.index('# Pesapal')
end = text.index('# Google Maps', start)
text = text[:start] + '''# Payments: IOTEC mobile money + cash on delivery
# All IOTEC credentials belong in the Go API environment, never here.
NEXT_PUBLIC_MOBILE_MONEY_ENABLED=true
NEXT_PUBLIC_BASE_URL=http://localhost:3000

# -------------------------------------------------------------------
''' + text[end:]
put(env, text)
env = api / '.env.example'
text = env.read_text(encoding='utf-8')
if 'GOOGLE_MAPS_SERVER_API=' not in text:
    text += '\n# Server-side Google Routes key for delivery pricing.\nGOOGLE_MAPS_SERVER_API=\n'
put(env, text)

readme = web / 'README.md'
text = readme.read_text(encoding='utf-8')
put(readme, '''> **Current payments:** IOTEC mobile money (MTN / Airtel) and cash on delivery.
> Pesapal checkout, callbacks and IPN registration are retired (HTTP 410).
> See [IOTEC setup](docs/iotec-payments.md) for the current configuration.
> Pesapal references in the original template documentation below are historical.

''' + text)
guide = '''# IOTEC payments

Online checkout uses IOTEC mobile money; cash on delivery remains available.
The integration follows NobliFi's server-side OAuth, collection and status-polling flow.

## Configure the Go API

Set `IOTEC_MOBILE_MONEY_ENABLED=true`, `IOTEC_CLIENT_ID`,
`IOTEC_CLIENT_SECRET`, and `IOTEC_WALLET_ID` for the Afrigoals merchant.
Keep these credentials server-side. Defaults are `https://pay.iotec.io` and
`https://id.iotec.io/connect/token`; the currency is UGX.
`GOOGLE_MAPS_SERVER_API` is required for authoritative delivery pricing.

Set `NEXT_PUBLIC_MOBILE_MONEY_ENABLED=true` in Next.js. This is a public feature
switch, not a payment credential. Restart the API and rebuild/restart Next.js
after configuration changes.

## Checkout and confirmation

1. Create the order with `paymentMethod: "mobile_money"` (or `"cod"`).
2. POST the payer phone to `/api/v1/orders/:id/payments/mobile-money`.
3. Approve the MTN/Airtel prompt. The pay page polls the same authenticated URL.
4. The API verifies the provider result, amount, currency, wallet and merchant
   reference before marking the order paid. A browser redirect is not proof of payment.

Pending attempts are reused; uncertain network outcomes stay in flight for
reconciliation rather than creating a second collection. Failed attempts can be
retried from the existing order payment page. Admin reconciliation is available
at `/api/v1/admin/payments/mobile-money/:id/reconcile` (POST).

Optional callbacks use `/api/v1/payments/iotec/callback` and require a matching
`X-IoTec-Callback-Secret` header configured through `IOTEC_CALLBACK_SECRET`.
Without that setup, confirmation continues through authenticated status polling.

## Retired provider

New Pesapal orders are rejected, and old checkout/callback/IPN URLs return 410.
Historical order columns are retained. Existing unpaid Pesapal orders are not
automatically recollected through IOTEC; review them before placing a new order.
No schema migration or live charge is required to install this change.
'''
put(web / 'docs/iotec-payments.md', guide)
put(api / 'docs/iotec-payments.md', guide)

# The earlier regression should now exercise only the supported checkout methods.
path = api / 'routes/orders_test.go'
text = path.read_text(encoding='utf-8')
text = '\n'.join(line for line in text.split('\n') if '{"pesapal", "/orders/checkout/pesapal"' not in line)
text += '''
func TestRetiredProviderCannotStartOrConfirmPayments(t *testing.T) {
    app := fiber.New()
    auth := func(c *fiber.Ctx) error {
        c.Locals("userID", "test-user")
        c.Locals("email", "customer@example.com")
        return c.Next()
    }
    RegisterOrders(app, auth, Deps{})
    RegisterPayments(app, Deps{})
    for _, path := range []string{"/orders/checkout/pesapal", "/orders/payment-callback", "/payments/pesapal/ipn"} {
        method := "POST"
        if path == "/orders/payment-callback" { method = "GET" }
        res, err := app.Test(httptest.NewRequest(method, path, nil))
        if err != nil { t.Fatal(err) }
        res.Body.Close()
        if res.StatusCode != 410 { t.Errorf("%s = %d; want 410", path, res.StatusCode) }
    }
    req := httptest.NewRequest("POST", "/orders", strings.NewReader(`{"items":[{"productId":"p1","quantity":1}],"deliveryAddress":"Kampala","paymentMethod":"pesapal"}`))
    req.Header.Set("Content-Type", "application/json")
    res, err := app.Test(req)
    if err != nil { t.Fatal(err) }
    defer res.Body.Close()
    if res.StatusCode != 400 { t.Fatalf("Pesapal order = %d; want 400", res.StatusCode) }
}
'''
put(path, text)

path = web / 'tests/auth-protected.test.ts'
text = path.read_text(encoding='utf-8')
text += '''
test("online checkout creates an IOTEC order and starts collection without a gateway redirect", async () => {
  setRequestCookies({ afrigoals_token: "customer-token" });
  const calls = mockFetch((url) => {
    if (url === `${BASE}/orders`) return new Response(JSON.stringify({ id: "o1", orderNumber: "AFG-1" }), { status: 201 });
    if (url === `${BASE}/orders/o1/payments/mobile-money`) return new Response(JSON.stringify({ status: "processing" }), { status: 202 });
    throw new Error(`Unexpected checkout URL: ${url}`);
  });
  const result = await createCheckoutSession(
    [{ productId: "p1", name: "Kit", price: 1000, quantity: 1 }],
    { address: "Kampala", lat: 0.3, lng: 32.5, contactName: "Customer", contactPhone: "+256771234567" },
  );
  assert.equal(result.success, true);
  assert.equal(result.url, "/orders/o1/pay");
  assert.equal(calls.length, 2);
  assert.equal(JSON.parse(String(calls[0].init?.body)).paymentMethod, "mobile_money");
  assert.equal(JSON.parse(String(calls[1].init?.body)).phone, "+256771234567");
});
'''
# Follow the project's cookie constant rather than assuming its spelling.
text = text.replace('import { createCheckoutSession }', 'import { AUTH_COOKIE_NAME } from "../lib/auth/constants";\nimport { createCheckoutSession }', 1)
text = text.replace('setRequestCookies({ afrigoals_token: "customer-token" });', 'setRequestCookies({ [AUTH_COOKIE_NAME]: "customer-token" });')
put(path, text)
print('Migrated online checkout to IOTEC; retained COD and historical order fields.')
