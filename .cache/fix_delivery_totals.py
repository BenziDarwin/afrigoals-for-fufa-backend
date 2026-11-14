from pathlib import Path
import json

api = Path('F:/Afrigoals/Afrigoals_ecommerce_api')
web = Path('F:/Afrigoals/Afrigoals_ecommerce')
def put(root, name, text):
    path = root / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding='utf-8')

put(api, 'models/delivery_settings.go', '''package models

import (
    "time"
    "gorm.io/datatypes"
)

// One store-wide configuration; orders retain their original priced totals.
type DeliverySettings struct {
    ID uint `gorm:"primaryKey"`
    Configuration datatypes.JSON `gorm:"not null"`
    UpdatedBy string
    UpdatedAt time.Time
}
''')
path = api / 'db/db.go'
text = path.read_text(encoding='utf-8').replace('&models.PaymentAttempt{},', '&models.PaymentAttempt{},\n        &models.DeliverySettings{},', 1)
put(api, 'db/db.go', text)

path = api / 'services/delivery.go'
text = path.read_text(encoding='utf-8')
names = ['OriginLat','OriginLng','BaseFee','MinimumFee','RoundToNearest','PetrolPerLitre','MotorcycleKmPerLitre','RoundTripFactor','RiderMarginPerKm','PeakMultiplier','ModerateMultiplier','OffPeakMultiplier','LongDistanceThresholdKm','LongDistanceFeeUGX']
import re
for name in names:
    text = re.sub(r'('+name+r'\s+float64)(?![^\n]*`)', r'\1 `json:"'+name[0].lower()+name[1:]+'"`', text, count=1)
text = text.replace('type DeliveryQuoter struct {', 'type DeliveryQuoter struct {\n    Mode string\n    FlatFee float64', 1)
needle = '\tif q.APIKey == "" {'
assert needle in text
text = text.replace(needle, '''    if q.Mode == "flat" {
        return DeliveryQuote{Fee: q.FlatFee, TrafficPeriod: "flat"}, nil
    }

''' + needle, 1)
put(api, 'services/delivery.go', text)

put(api, 'services/delivery_settings.go', '''package services

import (
    "encoding/json"
    "errors"
    "fmt"
    "math"
    "os"
    "strings"
    "afrigoals-api.com/models"
    "gorm.io/datatypes"
    "gorm.io/gorm"
    "gorm.io/gorm/clause"
)

type DeliverySettings struct {
    Mode string `json:"mode"`
    FlatFee float64 `json:"flatFee"`
    Pricing DeliveryPricing `json:"pricing"`
}

func DefaultDeliverySettings() DeliverySettings {
    mode := strings.TrimSpace(os.Getenv("DELIVERY_PRICING_MODE"))
    if mode != "flat" { mode = "distance" }
    return DeliverySettings{Mode: mode, FlatFee: envFloat("DELIVERY_FLAT_FEE", 300), Pricing: LoadDeliveryPricing()}
}

func ValidateDeliverySettings(s DeliverySettings) error {
    if s.Mode != "flat" && s.Mode != "distance" { return errors.New("choose flat or distance delivery pricing") }
    p := s.Pricing
    if err := validateCoordinates(p.OriginLat, p.OriginLng); err != nil { return err }
    for _, value := range []float64{s.FlatFee, p.BaseFee, p.MinimumFee, p.PetrolPerLitre,
        p.RoundTripFactor, p.RiderMarginPerKm, p.LongDistanceThresholdKm, p.LongDistanceFeeUGX} {
        if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1e9 { return errors.New("delivery rates must be finite non-negative numbers") }
    }
    for _, value := range []float64{p.RoundToNearest, p.MotorcycleKmPerLitre, p.PeakMultiplier, p.ModerateMultiplier, p.OffPeakMultiplier} {
        if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 || value > 1e9 { return errors.New("rounding, fuel efficiency and traffic multipliers must be positive") }
    }
    return nil
}

func LoadDeliverySettings(db *gorm.DB) (DeliverySettings, error) {
    if db == nil { return DefaultDeliverySettings(), nil }
    var row models.DeliverySettings
    err := db.First(&row, "id = ?", 1).Error
    if errors.Is(err, gorm.ErrRecordNotFound) { return DefaultDeliverySettings(), nil }
    if err != nil { return DeliverySettings{}, err }
    var settings DeliverySettings
    if err := json.Unmarshal(row.Configuration, &settings); err != nil { return settings, err }
    return settings, ValidateDeliverySettings(settings)
}

func SaveDeliverySettings(db *gorm.DB, settings DeliverySettings, updatedBy string) error {
    if err := ValidateDeliverySettings(settings); err != nil { return err }
    encoded, err := json.Marshal(settings)
    if err != nil { return err }
    row := models.DeliverySettings{ID: 1, Configuration: datatypes.JSON(encoded), UpdatedBy: updatedBy}
    return db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}},
        DoUpdates: clause.AssignmentColumns([]string{"configuration", "updated_by", "updated_at"}),
    }).Create(&row).Error
}

func ConfiguredDeliveryQuoter(db *gorm.DB, key string) (*DeliveryQuoter, error) {
    settings, err := LoadDeliverySettings(db)
    if err != nil { return nil, fmt.Errorf("%w: delivery settings unavailable", ErrDeliveryQuoteUnavailable) }
    q := NewDeliveryQuoter(key)
    q.Mode, q.FlatFee, q.Pricing = settings.Mode, settings.FlatFee, settings.Pricing
    return q, nil
}

func CheckExpectedOrderTotal(expected *float64, actual float64) error {
    if expected == nil { return nil }
    if math.IsNaN(*expected) || math.IsInf(*expected, 0) || *expected < 0 || math.Round(*expected) != math.Round(actual) {
        return errors.New("Your total has changed. Refresh checkout to review the updated prices before paying.")
    }
    return nil
}
''')

put(api, 'routes/delivery_pricing.go', '''package routes

import (
    "errors"
    "afrigoals-api.com/middleware"
    "afrigoals-api.com/services"
    "github.com/gofiber/fiber/v2"
)

func RegisterDeliveryPricing(router fiber.Router, auth fiber.Handler, deps Deps) {
    router.Post("/delivery/quote", func(c *fiber.Ctx) error {
        var input struct { Lat *float64 `json:"lat"`; Lng *float64 `json:"lng"` }
        if err := c.BodyParser(&input); err != nil || input.Lat == nil || input.Lng == nil {
            return c.Status(400).JSON(fiber.Map{"error": "Delivery coordinates are required"})
        }
        q, err := services.ConfiguredDeliveryQuoter(deps.DB, deps.Config.GoogleMapsServerAPIKey)
        if err != nil { return c.Status(503).JSON(fiber.Map{"error": "Delivery pricing is temporarily unavailable"}) }
        quote, err := q.Quote(c.UserContext(), *input.Lat, *input.Lng)
        if err != nil {
            code := 400
            if errors.Is(err, services.ErrDeliveryQuoteUnavailable) { code = 503 }
            return c.Status(code).JSON(fiber.Map{"error": "Could not calculate delivery for this location"})
        }
        c.Set("Cache-Control", "no-store")
        return c.JSON(fiber.Map{"fee": quote.Fee, "distanceKm": quote.DistanceKm,
            "distanceMeters": quote.DistanceMeters, "currency": "UGX"})
    })
    router.Get("/admin/delivery-pricing", auth, middleware.RequireAdmin(), func(c *fiber.Ctx) error {
        settings, err := services.LoadDeliverySettings(deps.DB)
        if err != nil { return c.Status(503).JSON(fiber.Map{"error": "Could not load delivery settings"}) }
        c.Set("Cache-Control", "no-store")
        return c.JSON(settings)
    })
    router.Put("/admin/delivery-pricing", auth, middleware.RequireAdmin(), func(c *fiber.Ctx) error {
        var settings services.DeliverySettings
        if err := c.BodyParser(&settings); err != nil { return c.Status(400).JSON(fiber.Map{"error": "Invalid delivery settings"}) }
        if err := services.ValidateDeliverySettings(settings); err != nil { return c.Status(400).JSON(fiber.Map{"error": err.Error()}) }
        userID, _ := c.Locals("userID").(string)
        if err := services.SaveDeliverySettings(deps.DB, settings, userID); err != nil { return c.Status(500).JSON(fiber.Map{"error": "Could not save delivery settings"}) }
        return c.JSON(settings)
    })
}
''')
path = api / 'routes/routes.go'
text = path.read_text(encoding='utf-8').replace('RegisterOrders(api, auth, deps)', 'RegisterDeliveryPricing(api, auth, deps)\n\tRegisterOrders(api, auth, deps)', 1)
put(api, 'routes/routes.go', text)
path = api / 'routes/orders.go'
text = path.read_text(encoding='utf-8')
old = '''quote, err := services.NewDeliveryQuoter(deps.Config.GoogleMapsServerAPIKey).Quote(
\t\tc.UserContext(),
\t\treq.DeliveryLat,
\t\treq.DeliveryLng,
\t)'''
assert old in text
text = text.replace(old, '''quoter, err := services.ConfiguredDeliveryQuoter(deps.DB, deps.Config.GoogleMapsServerAPIKey)
    if err != nil { return models.Order{}, nil, models.OrderStatusEvent{}, &apiFailure{status: 503, msg: "Delivery pricing is temporarily unavailable"} }
    quote, err := quoter.Quote(c.UserContext(), req.DeliveryLat, req.DeliveryLng)''', 1)
needle = '\torder.Total = subtotal + order.DeliveryFee'
text = text.replace(needle, needle + '''
    if err := services.CheckExpectedOrderTotal(req.ExpectedTotal, order.Total); err != nil {
        tx.Rollback()
        return models.Order{}, nil, models.OrderStatusEvent{}, &apiFailure{status: fiber.StatusConflict, msg: err.Error()}
    }
''', 1)
put(api, 'routes/orders.go', text)
path = api / 'dtos/order.go'
text = path.read_text(encoding='utf-8').replace('type CreateOrderRequest struct {', 'type CreateOrderRequest struct {\n    ExpectedTotal *float64 `json:"expectedTotal"`', 1).replace('// cod|pesapal', '// cod|mobile_money')
put(api, 'dtos/order.go', text)

# Both quote display and order creation now use the Go API settings.
put(web, 'lib/actions/delivery.ts', '''"use server";

import { getBackendApiBaseUrl } from "@/lib/api/backend";

export interface DeliveryQuoteResult {
  success: boolean;
  fee?: number;
  distanceKm?: number;
  distanceMeters?: number;
  currency?: "UGX";
  error?: string;
}

export async function getDeliveryQuote(input: { lat: number; lng: number }): Promise<DeliveryQuoteResult> {
  if (!Number.isFinite(input.lat) || !Number.isFinite(input.lng) || Math.abs(input.lat) > 90 || Math.abs(input.lng) > 180) {
    return { success: false, error: "Invalid delivery coordinates" };
  }
  try {
    const response = await fetch(`${getBackendApiBaseUrl()}/delivery/quote`, {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input), cache: "no-store",
    });
    const data = await response.json();
    if (!response.ok) return { success: false, error: data.error || "Could not calculate delivery" };
    if (!Number.isFinite(data.fee) || data.fee < 0 || !Number.isFinite(data.distanceKm)) {
      return { success: false, error: "Invalid delivery quote" };
    }
    return { success: true, fee: data.fee, distanceKm: data.distanceKm, distanceMeters: data.distanceMeters, currency: "UGX" };
  } catch {
    return { success: false, error: "Delivery pricing is temporarily unavailable. Please try again." };
  }
}
''')
path = web / 'lib/actions/create-cash-on-delivery-order.ts'
text = path.read_text(encoding='utf-8').replace('  payerPhone?: string,\n', '  payerPhone?: string,\n  expectedTotal?: number,\n', 1)
text = text.replace('      paymentMethod,\n    };', '      paymentMethod,\n      expectedTotal,\n    };', 1)
put(web, 'lib/actions/create-cash-on-delivery-order.ts', text)
path = web / 'components/app/CheckoutButton.tsx'
text = path.read_text(encoding='utf-8').replace('  disabled?: boolean;', '  disabled?: boolean;\n  expectedTotal: number;', 1)
text = text.replace('  disabled,\n', '  disabled,\n  expectedTotal,\n', 1)
text = text.replace('          payerPhone,\n        );', '          payerPhone,\n          expectedTotal,\n        );', 1)
put(web, 'components/app/CheckoutButton.tsx', text)
path = web / 'app/(app)/checkout/CheckoutClient.tsx'
text = path.read_text(encoding='utf-8').replace('<CheckoutButton\n', '<CheckoutButton\n                  expectedTotal={total}\n', 1)
put(web, 'app/(app)/checkout/CheckoutClient.tsx', text)

put(web, 'app/api/admin/delivery-pricing/route.ts', '''import { NextResponse } from "next/server";
import { authedFetch } from "@/lib/api/proxy";

export async function GET() {
  const response = await authedFetch("/admin/delivery-pricing");
  return new NextResponse(await response.text(), { status: response.status, headers: { "Content-Type": "application/json", "Cache-Control": "no-store" } });
}
export async function PUT(request: Request) {
  const response = await authedFetch("/admin/delivery-pricing", {
    method: "PUT", headers: { "Content-Type": "application/json" }, body: await request.text(),
  });
  return new NextResponse(await response.text(), { status: response.status, headers: { "Content-Type": "application/json" } });
}
''')
put(web, 'app/(admin)/admin/delivery-pricing/page.tsx', '''"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";

const fields = [
  ["originLat", "Shop latitude"], ["originLng", "Shop longitude"],
  ["baseFee", "Base fee (UGX)"], ["minimumFee", "Minimum fee (UGX)"],
  ["roundToNearest", "Round to nearest (UGX)"],
  ["petrolPerLitre", "Petrol price per litre (UGX)"],
  ["motorcycleKmPerLitre", "Motorcycle kilometres per litre"],
  ["roundTripFactor", "Round trip factor"], ["riderMarginPerKm", "Rider margin per kilometre (UGX)"],
  ["peakMultiplier", "Peak traffic multiplier"], ["moderateMultiplier", "Moderate traffic multiplier"],
  ["offPeakMultiplier", "Off-peak traffic multiplier"],
  ["longDistanceThresholdKm", "Long-distance threshold (km)"],
  ["longDistanceFeeUGX", "Long-distance flat fee (UGX)"],
] as const;
type Settings = { mode: "flat" | "distance"; flatFee: number; pricing: Record<string, number> };

export default function DeliveryPricingPage() {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [saving, setSaving] = useState(false);

  async function load() {
    setError("");
    try {
      const response = await fetch("/api/admin/delivery-pricing", { cache: "no-store" });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || "Could not load delivery pricing");
      setSettings(data);
    } catch (cause) { setError(cause instanceof Error ? cause.message : "Could not load delivery pricing"); }
  }
  useEffect(() => { void load(); }, []);

  async function save(event: React.FormEvent) {
    event.preventDefault();
    setSaving(true); setError(""); setMessage("");
    try {
      const response = await fetch("/api/admin/delivery-pricing", {
        method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(settings),
      });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || "Could not save delivery pricing");
      setSettings(data);
      setMessage("Delivery pricing saved. New quotes and orders now use these settings.");
    } catch (cause) { setError(cause instanceof Error ? cause.message : "Could not save delivery pricing"); }
    finally { setSaving(false); }
  }
  const inputClass = "mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-zinc-950 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100";
  return <div className="mx-auto max-w-4xl space-y-6 p-6">
    <div><h1 className="text-2xl font-semibold">Delivery pricing</h1>
      <p className="mt-2 text-sm text-zinc-500">Choose a flat fee for testing or distance-based rates. Changes apply to new quotes and orders; existing orders keep their original totals.</p></div>
    {error && <div role="alert" className="rounded-lg bg-red-50 p-4 text-red-700">{error}</div>}
    {message && <div role="status" className="rounded-lg bg-green-50 p-4 text-green-800">{message}</div>}
    {!settings ? <Button onClick={() => void load()}>Reload settings</Button> :
      <form onSubmit={save} className="space-y-6">
        <fieldset disabled={saving} className="space-y-6 rounded-xl border bg-white p-6 dark:bg-zinc-950">
          <label className="block text-sm font-medium" htmlFor="delivery-mode">Pricing method
            <select id="delivery-mode" className={inputClass} value={settings.mode} onChange={event => { setMessage(""); setSettings({ ...settings, mode: event.target.value as Settings["mode"] }); }}>
              <option value="flat">Flat fee (testing)</option><option value="distance">Distance-based pricing</option>
            </select>
          </label>
          {settings.mode === "flat" && <label className="block text-sm font-medium" htmlFor="flat-fee">Delivery fee (UGX)
            <input id="flat-fee" type="number" required min="0" step="1" className={inputClass} value={settings.flatFee} onChange={event => setSettings({ ...settings, flatFee: event.target.valueAsNumber })} />
          </label>}
          <div><h2 className="font-semibold">Distance-based rates</h2><p className="mt-1 text-sm text-zinc-500">These rates take effect when distance-based pricing is selected. The fee uses the route distance, fuel cost, rider margin and traffic multiplier, with a minimum fee and rounding.</p></div>
          <div className="grid gap-4 sm:grid-cols-2">{fields.map(([key, label]) =>
            <label key={key} htmlFor={key} className="block text-sm font-medium">{label}
              <input id={key} type="number" required step="any" min={key === "originLat" || key === "originLng" ? undefined : 0} className={inputClass} value={settings.pricing[key] ?? ""} onChange={event => setSettings({ ...settings, pricing: { ...settings.pricing, [key]: event.target.valueAsNumber } })} />
            </label>
          )}</div>
          <Button type="submit">{saving ? "Saving…" : "Save delivery pricing"}</Button>
        </fieldset>
      </form>}
  </div>;
}
''')
path = web / 'app/(admin)/admin/layout.tsx'
text = path.read_text(encoding='utf-8').replace('{ label: "Payments", href: "/admin/payments", icon: CreditCard },', '{ label: "Payments", href: "/admin/payments", icon: CreditCard },\n  { label: "Delivery pricing", href: "/admin/delivery-pricing", icon: Package },', 1)
put(web, 'app/(admin)/admin/layout.tsx', text)

# Testing uses an explicit API-owned flat fee until an administrator saves settings.
path = api / '.env'
text = path.read_text(encoding='utf-8')
for key, value in [('DELIVERY_PRICING_MODE', 'flat'), ('DELIVERY_FLAT_FEE', '300')]:
    if re.search(r'(?m)^'+key+r'=', text): text = re.sub(r'(?m)^'+key+r'=[^\n]*', key+'='+value, text)
    else: text += '\n'+key+'='+value+'\n'
put(api, '.env', text)
path = api / '.env.example'
text = path.read_text(encoding='utf-8') + '\n# Initial pricing before the admin saves store-wide settings.\nDELIVERY_PRICING_MODE=flat\nDELIVERY_FLAT_FEE=300\n'
put(api, '.env.example', text)
print('Added API-owned quotes, persistent admin delivery pricing, flat UGX 300 test mode and pre-payment total validation.')
