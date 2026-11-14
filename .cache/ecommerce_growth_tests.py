from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def put(root,name,text):
 p=root/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text,encoding='utf-8')
put(api,'services/iotec_wallet_test.go','''package services
import("context";"encoding/json";"net/http";"strings";"testing")
func TestWalletBalanceUsesProviderActualBalanceAndRedactsSecrets(t *testing.T){
 client,_,done:=newMockIoTec(t,func(w http.ResponseWriter,r *http.Request){
  if r.URL.Path!="/api/wallet-balance/W1"{t.Errorf("wrong wallet path %s",r.URL.Path)}
  w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"id":"W1","name":"Store wallet","currency":"UGX","actualBalance":12500,"availableBalance":999,"callBackSecurityHeaderValue":"provider-secret"}`))
 });defer done();balance,err:=client.WalletBalance(context.Background());if err!=nil{t.Fatal(err)}
 if balance.ActualBalance==nil||*balance.ActualBalance!=12500{t.Fatal("did not use actual balance")}
 raw,_:=json.Marshal(balance);if strings.Contains(string(raw),"provider-secret")||strings.Contains(string(raw),"availableBalance"){t.Fatal("provider credentials or deprecated fields leaked")}
}
func TestMissingWalletBalanceIsNotDisplayedAsZero(t *testing.T){
 client,_,done:=newMockIoTec(t,func(w http.ResponseWriter,r *http.Request){_,_=w.Write([]byte(`{"id":"W1","currency":"UGX"}`))});defer done()
 if _,err:=client.WalletBalance(context.Background());err==nil{t.Fatal("missing provider balance accepted as zero")}
}
''')
put(api,'routes/commerce_growth_test.go','''package routes
import("net/http/httptest";"strings";"testing";"github.com/gofiber/fiber/v2")
func TestCommerceGrowthAdminRoutesRejectCustomers(t *testing.T){
 app:=fiber.New();auth:=func(c *fiber.Ctx)error{c.Locals("role","user");return c.Next()};RegisterCommerceGrowth(app,auth,Deps{});RegisterEmailCampaigns(app,auth,Deps{})
 for _,tt:=range []struct{method,path string}{{"GET","/admin/iotec/wallet"},{"GET","/admin/commerce/overview"},{"POST","/admin/assistant"},{"PUT","/admin/partner-stores"},{"GET","/admin/email-campaigns"},{"PUT","/admin/email-automation"}}{
  response,err:=app.Test(httptest.NewRequest(tt.method,tt.path,strings.NewReader(`{}`)));if err!=nil{t.Fatal(err)};response.Body.Close();if response.StatusCode!=403{t.Fatalf("%s %s returned %d",tt.method,tt.path,response.StatusCode)}
 }
}
func TestPartnerStoreLinksRejectExecutableSchemes(t *testing.T){
 for _,link:=range []string{"javascript:alert(1)","data:text/html,x","ftp://example.com"}{if validStoreURL(link){t.Fatal("unsafe URL accepted",link)}}
 if !validStoreURL("https://www.instagram.com/example")||!validStoreURL(""){t.Fatal("valid directory URL rejected")}
}
''')
p=api/'routes/release_database_test.go';s=p.read_text(encoding='utf-8').replace('"encoding/json"','"encoding/json"\n"io"\n"net/http"',1)
s=s.replace('&models.DeliverySettings{});', '&models.DeliverySettings{}, &models.PaymentAttempt{}, &models.EmailCampaign{}, &models.EmailDelivery{}, &models.EmailPreference{}, &models.EmailAutomation{}, &models.PartnerStore{});',1)
needle='\n}\n';index=s.rfind(needle)
assert index>0
s=s[:index]+'''
 t.Run("commerce reports reconcile actual paid orders",func(t *testing.T){
  overview,err:=services.BuildCommerceOverview(tx);if err!=nil{t.Fatal(err)}
  if overview.Report.Totals.ProductSales!=1100||overview.Report.Totals.DeliveryFees!=300{t.Fatal("incorrect paid totals",overview.Report.Totals)}
  if len(overview.Performance)!=2||len(overview.Daily)!=1{t.Fatalf("missing catalog or daily reporting: %d %d",len(overview.Performance),len(overview.Daily))}
 })
 t.Run("campaigns exclude unsubscribers and send once with tracking",func(t *testing.T){
  if err:=tx.Create(&models.EmailAutomation{ID:1,Enabled:true}).Error;err!=nil{t.Fatal(err)}
  if err:=tx.Create(&models.EmailPreference{UserID:other.ID,Token:uuid.NewString(),Unsubscribed:true}).Error;err!=nil{t.Fatal(err)}
  due:=time.Now().Add(-time.Minute);campaign:=models.EmailCampaign{Subject:"Release test",Body:"<b>Offer</b>\\nLine 2",Status:"scheduled",ScheduledAt:&due};if err:=tx.Create(&campaign).Error;err!=nil{t.Fatal(err)}
  calls:=0;keys:=map[string]bool{}
  sender:=&services.EmailService{APIKey:"test-only",FromName:"Test",FromEmail:"test@release.test",HTTPClient:&http.Client{Transport:releaseMailTransport(func(r *http.Request)(*http.Response,error){
   calls++;key:=r.Header.Get("Idempotency-Key");if key==""||keys[key]{t.Fatal("missing or duplicate delivery idempotency key")};keys[key]=true
   var payload struct{HTML string `json:"html"`;To []string `json:"to"`};if err:=json.NewDecoder(r.Body).Decode(&payload);err!=nil{t.Fatal(err)}
   if strings.Contains(payload.HTML,"<b>")||!strings.Contains(payload.HTML,"&lt;b&gt;")||!strings.Contains(payload.HTML,`href="https://api.example.test/api/v1/email/unsubscribe/`){t.Fatal("campaign HTML or unsubscribe link malformed",payload.HTML)}
   if payload.To[0]==other.Email{t.Fatal("unsubscribed client was emailed")}
   return &http.Response{StatusCode:200,Body:io.NopCloser(strings.NewReader(`{"id":"mock-delivery"}`)),Header:make(http.Header)},nil
  })}}
  if err:=services.ProcessEmailCampaigns(tx,sender,"https://api.example.test");err!=nil{t.Fatal(err)}
  if calls!=2{t.Fatalf("expected owner and vendor deliveries, got %d",calls)}
  if err:=services.ProcessEmailCampaigns(tx,sender,"https://api.example.test");err!=nil{t.Fatal(err)}
  if calls!=2{t.Fatal("campaign sent twice")}
  var sent int64;if err:=tx.Model(&models.EmailDelivery{}).Where("status = ?","sent").Count(&sent).Error;err!=nil||sent!=2{t.Fatalf("delivery tracking wrong: %d, %v",sent,err)}
 })
''' + s[index:]
s+='''
type releaseMailTransport func(*http.Request)(*http.Response,error)
func(f releaseMailTransport)RoundTrip(r *http.Request)(*http.Response,error){return f(r)}
''';put(api,'routes/release_database_test.go',s)
put(web,'tests/commerce-growth.test.ts','''import"./setup-server-only";
import assert from"node:assert/strict";import{afterEach,mock,test}from"node:test";import{GET as wallet}from"../app/api/admin/iotec/wallet/route";import{PUT as campaign}from"../app/api/admin/email-campaigns/route";import{PUT as partner}from"../app/api/admin/partner-stores/route";import{POST as chat}from"../app/api/chat/route";import{mockFetch,json,setRequestCookies}from"./helpers";import{AUTH_COOKIE_NAME}from"../lib/auth/constants";
afterEach(()=>mock.restoreAll());
test("wallet and campaign proxies reject guests before reaching the backend",async()=>{setRequestCookies({});const calls=mockFetch(()=>json({}));assert.equal((await wallet()).status,401);assert.equal((await campaign(new Request("http://localhost/api/admin/email-campaigns",{method:"PUT",body:"{}"}))).status,401);assert.equal(calls.length,0)});
test("partner store changes are forwarded using the admin session",async()=>{setRequestCookies({[AUTH_COOKIE_NAME]:"admin-token"});const calls=mockFetch(()=>json({name:"Vargo Sports"}));assert.equal((await partner(new Request("http://localhost/api/admin/partner-stores",{method:"PUT",body:JSON.stringify({name:"Vargo Sports",keywords:"Manchester United, Arsenal"})}))).status,200);assert.ok(calls[0].url.endsWith("/admin/partner-stores"))});
test("shopping chat forwards only text history and the current session to the scout-backed API",async()=>{setRequestCookies({[AUTH_COOKIE_NAME]:"owner-token"});const calls=mockFetch(()=>json({answer:"Contact the admin-listed partner store to confirm availability."}));const response=await chat(new Request("http://localhost/api/chat",{method:"POST",body:JSON.stringify({userId:"forged-account",messages:[{id:"1",role:"user",parts:[{type:"text",text:"Where can I find Arsenal jerseys?"}]}]})}));const stream=await response.text();assert.ok(stream.includes("admin-listed"));assert.ok(calls[0].url.endsWith("/assistant/chat"));assert.equal(JSON.stringify(JSON.parse(String(calls[0].init?.body))).includes("forged-account"),false)});
''')
import json
p=web/'package.json';data=json.loads(p.read_text(encoding='utf-8'));data['scripts']['test']+=' tests/commerce-growth.test.ts';put(web,'package.json',json.dumps(data,indent=2)+'\n')
print('Added wallet privacy, commerce authorization, scout chat and idempotent campaign tests.')
