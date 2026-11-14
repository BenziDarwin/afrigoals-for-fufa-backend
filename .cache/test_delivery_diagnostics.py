from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
p=api/'services/delivery.go';s=p.read_text();a='fmt.Errorf("%w: %v", ErrDeliveryQuoteUnavailable, err)';assert a in s;s=s.replace(a,'fmt.Errorf("%w: %w", ErrDeliveryQuoteUnavailable, err)');p.write_text(s,encoding='utf-8')
(api/'services/delivery_failure_test.go').write_text('''package services

import (
 "context"
 "errors"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
)

func TestDeliveryFailureClassifiesProviderWithoutLeakingSecrets(t *testing.T) {
 for _,tc:=range []struct{reason string;code string}{
  {"SERVICE_DISABLED","maps_routes_api_disabled"},
  {"BILLING_DISABLED","maps_billing_disabled"},
  {"API_KEY_HTTP_REFERRER_BLOCKED","maps_server_key_restricted"},
  {"API_KEY_IP_ADDRESS_BLOCKED","maps_server_key_restricted"},
  {"UNRECOGNIZED_SECRET","maps_routes_access_denied"},
 } {
  t.Run(tc.reason,func(t *testing.T){
   server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
    w.WriteHeader(403)
    w.Write([]byte(`{"error":{"status":"PERMISSION_DENIED","message":"secret-api-key-12345","details":[{"reason":"`+tc.reason+`","metadata":{"key":"secret-api-key-12345"}}]}}`))
   }));defer server.Close()
   q:=NewDeliveryQuoter("secret-api-key-12345");q.RoutesURL=server.URL
   _,err:=q.Quote(context.Background(),0.333,32.568)
   if !errors.Is(err,ErrDeliveryQuoteUnavailable)||DeliveryFailureCode(err)!=tc.code{t.Fatal("lost provider failure classification",DeliveryFailureCode(err))}
   safe:=SafeDeliveryFailure(err)
   if strings.Contains(safe,"secret-api-key")||strings.Contains(safe,"UNRECOGNIZED_SECRET"){t.Fatal("unsafe diagnostic",safe)}
  })
 }
}

func TestDeliveryFlatModeNeverNeedsGoogleServerKey(t *testing.T){
 q:=NewDeliveryQuoter("");q.Mode="flat";q.FlatFee=300
 q.RoutesURL="http://127.0.0.1:1/unreachable"
 quote,err:=q.Quote(context.Background(),0.333,32.568)
 if err!=nil||quote.Fee!=300{t.Fatal(quote,err)}
 q.Mode="distance"
 if _,err=q.Quote(context.Background(),0.333,32.568);DeliveryFailureCode(err)!="maps_server_key_missing"{t.Fatal("missing key not diagnosed",err)}
}
''',encoding='utf-8')
p=api/'routes/delivery_pricing_test.go';s=p.read_text();s=s.replace('"encoding/json"','"encoding/json"\n"afrigoals-api.com/services"');s+='''
func TestDeliveryPricingRejectsDistanceModeWithoutServerKey(t *testing.T){
 app:=fiber.New()
 RegisterDeliveryPricing(app,func(c *fiber.Ctx)error{c.Locals("role","admin");return c.Next()},Deps{})
 settings:=services.DefaultDeliverySettings();settings.Mode="distance"
 body,_:=json.Marshal(settings)
 req:=httptest.NewRequest("PUT","/admin/delivery-pricing",strings.NewReader(string(body)));req.Header.Set("Content-Type","application/json")
 res,err:=app.Test(req);if err!=nil{t.Fatal(err)};defer res.Body.Close()
 var data map[string]any;json.NewDecoder(res.Body).Decode(&data)
 if res.StatusCode!=503||data["code"]!="maps_server_key_missing"{t.Fatal(res.StatusCode,data)}
}
''';p.write_text(s,encoding='utf-8')
p=web/'tests/delivery-pricing.test.ts'
with p.open('a',encoding='utf-8') as f:f.write('''
test("checkout retains the API diagnostic code for failed production routing",async()=>{
 mockFetch(()=>json({error:"Could not calculate delivery for this location",code:"maps_routes_api_disabled"},503));
 const result=await getDeliveryQuote({lat:0.333,lng:32.568});
 assert.equal(result.success,false);assert.equal(result.errorCode,"maps_routes_api_disabled");
});

test("checkout handles missing quote routes without a JSON parsing crash",async()=>{
 mockFetch(()=>new Response("Cannot POST /api/v1/delivery/quote",{status:404}));
 const result=await getDeliveryQuote({lat:0.333,lng:32.568});
 assert.equal(result.success,false);assert.equal(result.errorCode,"invalid_delivery_response");
 assert.doesNotMatch(result.error??"",/Unexpected token/);
});
''')
