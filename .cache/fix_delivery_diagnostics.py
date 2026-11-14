from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def edit(p,a,b):
 s=p.read_text(encoding='utf-8');assert a in s,(str(p),a[:90]);p.write_text(s.replace(a,b),encoding='utf-8')
(api/'services/delivery_failure.go').write_text('''package services

import (
 "encoding/json"
 "errors"
 "fmt"
)

// Only allowlisted machine-readable fields reach logs or API responses.
// Google response messages can contain keys, URLs or account information.
type DeliveryFailure struct {
 Code string
 ProviderHTTP int
 ProviderStatus string
 ProviderReason string
}
func (e *DeliveryFailure) Error() string { return fmt.Sprintf("%s provider_http=%d provider_status=%s provider_reason=%s",e.Code,e.ProviderHTTP,e.ProviderStatus,e.ProviderReason) }
func (e *DeliveryFailure) Unwrap() error { return ErrDeliveryQuoteUnavailable }
func DeliveryFailureCode(err error) string {
 var failure *DeliveryFailure
 if errors.As(err,&failure){return failure.Code}
 return "delivery_quote_unavailable"
}
func SafeDeliveryFailure(err error) string {
 var failure *DeliveryFailure
 if errors.As(err,&failure){return failure.Error()}
 return "delivery_quote_unavailable"
}

func googleDeliveryFailure(status int,raw []byte) error {
 failure:=&DeliveryFailure{Code:"maps_routes_request_failed",ProviderHTTP:status}
 var body struct { Error struct { Status string `json:"status"`; Details []struct {Reason string `json:"reason"`} `json:"details"` } `json:"error"` }
 _=json.Unmarshal(raw,&body)
 switch body.Error.Status {case "PERMISSION_DENIED","UNAUTHENTICATED","INVALID_ARGUMENT","RESOURCE_EXHAUSTED","UNAVAILABLE","INTERNAL","NOT_FOUND":failure.ProviderStatus=body.Error.Status}
 switch status {case 401,403:failure.Code="maps_routes_access_denied";case 429:failure.Code="maps_routes_rate_limited";case 400:failure.Code="maps_routes_request_rejected";case 500,502,503,504:failure.Code="maps_routes_unavailable"}
 for _,detail:=range body.Error.Details {
  switch detail.Reason {
  case "SERVICE_DISABLED":failure.Code="maps_routes_api_disabled";failure.ProviderReason=detail.Reason
  case "BILLING_DISABLED","BILLING_NOT_ACTIVE":failure.Code="maps_billing_disabled";failure.ProviderReason=detail.Reason
  case "API_KEY_INVALID","API_KEY_EXPIRED","API_KEY_HTTP_REFERRER_BLOCKED","API_KEY_IP_ADDRESS_BLOCKED","API_KEY_SERVICE_BLOCKED":failure.Code="maps_server_key_restricted";failure.ProviderReason=detail.Reason
  }
 }
 return failure
}
''',encoding='utf-8')
p=api/'services/delivery.go'
edit(p,'fmt.Errorf("%w: GOOGLE_MAPS_SERVER_API is not configured", ErrDeliveryQuoteUnavailable)','&DeliveryFailure{Code:"maps_server_key_missing"}')
edit(p,'return 0, fmt.Errorf("routes API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))','return 0, googleDeliveryFailure(resp.StatusCode, raw)')
# Classify network/read/invalid response failures without retaining transport URLs.
edit(p,'resp, err := client.Do(req)\n\tif err != nil {\n\t\treturn 0, err\n\t}','resp, err := client.Do(req)\n\tif err != nil {\n\t\treturn 0, &DeliveryFailure{Code:"maps_routes_unreachable"}\n\t}')
edit(p,'raw, err := io.ReadAll(resp.Body)','raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))')
edit(p,'return 0, errors.New("routes API returned no valid distance")','return 0, &DeliveryFailure{Code:"delivery_route_not_found"}')
p=api/'routes/delivery_pricing.go'
edit(p,'"errors"','"errors"\n"log"\n"strings"')
edit(p,'return c.Status(503).JSON(fiber.Map{"error": "Delivery pricing is temporarily unavailable"})','log.Printf("delivery_quote_failed code=delivery_settings_unavailable")\n return c.Status(503).JSON(fiber.Map{"error": "Delivery pricing is temporarily unavailable", "code":"delivery_settings_unavailable"})')
edit(p,'return c.Status(code).JSON(fiber.Map{"error": "Could not calculate delivery for this location"})','log.Printf("delivery_quote_failed mode=%s %s", q.Mode, services.SafeDeliveryFailure(err))\n return c.Status(code).JSON(fiber.Map{"error": "Could not calculate delivery for this location", "code":services.DeliveryFailureCode(err)})')
needle='\t\tuserID, _ := c.Locals("userID").(string)'
edit(p,needle,'''        if settings.Mode == "distance" && strings.TrimSpace(deps.Config.GoogleMapsServerAPIKey) == "" {
            return c.Status(503).JSON(fiber.Map{"error":"Distance pricing requires GOOGLE_MAPS_SERVER_API on the API server. Configure it or select flat pricing.", "code":"maps_server_key_missing"})
        }
'''+needle)
p=web/'lib/actions/delivery.ts'
edit(p,'  error?: string;','  error?: string;\n  errorCode?: string;')
edit(p,'    const data = await response.json();','''    const data = await response.json().catch(() => null);
    if (!data || typeof data !== "object") return {success:false,error:"Delivery pricing returned an unexpected response. Please try again.",errorCode:"invalid_delivery_response"};''')
edit(p,'        error: data.error || "Could not calculate delivery",','        error: typeof data.error === "string" ? data.error : "Could not calculate delivery",\n        errorCode: typeof data.code === "string" ? data.code : "delivery_quote_unavailable",')
edit(p,'      !Number.isFinite(data.distanceKm)','      !Number.isFinite(data.distanceKm) || data.distanceKm < 0 || (data.currency !== undefined && data.currency !== "UGX")')
with (api/'docs/delivery-pricing.md').open('a',encoding='utf-8') as f:f.write('''

## Production quote failures

The browser map key is distinct from the API server's GOOGLE_MAPS_SERVER_API. A working map does not prove the server can call Google Routes. Flat pricing bypasses Google entirely. The local testing defaults are DELIVERY_PRICING_MODE=flat and DELIVERY_FLAT_FEE=300; persisted admin settings take precedence over environment defaults.

Quote errors now return a safe code and log delivery_quote_failed without raw Google messages, API keys or destination coordinates. maps_server_key_missing means the API process has no server key. maps_routes_api_disabled means Routes API must be enabled for that key's Google project. maps_server_key_restricted means the key or its allowed API/IP restrictions prevent server requests. maps_billing_disabled means the project's billing needs attention. maps_routes_unreachable indicates network/timeout problems; delivery_route_not_found indicates no valid road distance was returned. Check the service logs rather than the browser Maps warnings. Enabling distance pricing is rejected if the server key is missing. Pricing is never silently replaced with an unverified fee.
''')
