package routes

import (
 "context"
 "encoding/json"
 "fmt"
 "net/http"
 "net/http/httptest"
 "os"
 "strings"
 "sync/atomic"
 "testing"
 "time"

 "afrigoals-api.com/models"
 "afrigoals-api.com/services"
 "github.com/gofiber/fiber/v2"
 "github.com/google/uuid"
 "github.com/joho/godotenv"
 "gorm.io/driver/postgres"
 "gorm.io/gorm"
 "gorm.io/gorm/logger"
)

func TestRefundDatabaseFlows(t *testing.T) {
 if os.Getenv("TEST_RELEASE_DB")!="true"{t.Skip("opt-in isolated development database test")}
 _=godotenv.Load("../.env");cfg:=services.LoadConfig()
 db,err:=gorm.Open(postgres.New(postgres.Config{DSN:cfg.DBPostgresDSN,PreferSimpleProtocol:true}),&gorm.Config{Logger:logger.Default.LogMode(logger.Silent)})
 if err!=nil{t.Fatal("database unavailable")}
 tx:=db.Begin();defer tx.Rollback()
 schema:="refund_test_"+strings.ReplaceAll(uuid.NewString(),"-","")
 if err:=tx.Exec("CREATE SCHEMA "+schema).Error;err!=nil{t.Fatal(err)}
 if err:=tx.Exec("SET LOCAL search_path TO "+schema+", public").Error;err!=nil{t.Fatal(err)}
 if err:=tx.AutoMigrate(&models.Order{},&models.OrderItem{},&models.OrderStatusEvent{},&models.Product{},&models.OrderDeliveryAssignment{},&models.PaymentAttempt{},&models.PaymentRefund{});err!=nil{t.Fatal(err)}
 now:=time.Now().UTC();userID:=uuid.NewString();wallet:="wallet-1"
 order:=models.Order{OrderNumber:"REFUND-TEST",UserID:userID,Email:"refund@test.invalid",Status:"paid",PaymentStatus:"paid",PaymentMethod:"mobile_money",Currency:"UGX",Subtotal:1700,DeliveryFee:300,Total:2000,PaidAt:&now}
 if err:=tx.Create(&order).Error;err!=nil{t.Fatal(err)}
 attempt:=models.PaymentAttempt{OrderID:order.ID,UserID:userID,MerchantReference:"original-1",ProviderReference:"original-tx",Amount:2000,Currency:"UGX",Status:"succeeded",Provider:"iotec",VerifiedAt:&now}
 if err:=tx.Create(&attempt).Error;err!=nil{t.Fatal(err)}
 var posts atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  w.Header().Set("Content-Type","application/json")
  switch {
  case r.URL.Path=="/token":fmt.Fprint(w,`{"access_token":"mock","expires_in":300}`)
  case r.URL.Path=="/api/collections/status/original-tx":fmt.Fprintf(w,`{"id":"original-tx","externalId":"original-1","walletId":"wallet-1","currency":"UGX","amount":2000,"payer":"256771234567","status":"Success"}`)
  case r.URL.Path=="/api/disbursements/disburse":posts.Add(1);fmt.Fprint(w,`{"status":"Pending"}`)
  case strings.HasPrefix(r.URL.Path,"/api/disbursements/external-id/"):
   id:=strings.TrimPrefix(r.URL.Path,"/api/disbursements/external-id/");var row models.PaymentRefund
   if tx.First(&row,"id = ?",id).Error!=nil{w.WriteHeader(404);return}
   fmt.Fprintf(w,`{"id":"refund-%s","externalId":"%s","walletId":"wallet-1","currency":"UGX","amount":%d,"payee":"256771234567","status":"Success"}`,id,id,row.Amount)
  default:t.Errorf("unexpected provider path %s",r.URL.Path);w.WriteHeader(404)
  }
 }));defer server.Close()
 cfg.IoTecBaseURL=server.URL;cfg.IoTecTokenURL=server.URL+"/token";cfg.IoTecWalletID=wallet;cfg.IoTecClientID="mock";cfg.IoTecClientSecret="mock";cfg.IoTecMobileMoneyEnabled=true
 role:="admin";auth:=func(c *fiber.Ctx)error{c.Locals("role",role);c.Locals("userID",userID);return c.Next()}
 app:=fiber.New();RegisterRefunds(app,auth,Deps{DB:tx,Config:cfg,IoTec:services.NewIoTecClient(cfg)})
 request:=func(method,path string,body any)(int,[]byte){
  bytes,_:=json.Marshal(body);req:=httptest.NewRequest(method,path,strings.NewReader(string(bytes)));req.Header.Set("Content-Type","application/json")
  res,err:=app.Test(req,30000);if err!=nil{t.Fatal(err)};defer res.Body.Close();var value json.RawMessage;json.NewDecoder(res.Body).Decode(&value);return res.StatusCode,value
 }
 makeDraft:=func(amount int,phone,kind string)map[string]any{return map[string]any{"transactionId":"original-tx","phone":phone,"customerName":"Customer","amount":amount,"kind":kind,"reason":"Customer cancellation","settlementNotes":"Customer agreed a partial refund for the returned item"}}
 role="user";if code,_:=request("GET","/admin/refunds",nil);code!=403{t.Fatal("customer can access refunds",code)};role="admin"
 if code,_:=request("POST","/admin/refunds",makeDraft(600,"0781234567","cancellation"));code!=400{t.Fatal("wrong original payer accepted",code)}
 code,body:=request("POST","/admin/refunds",makeDraft(1200,"0771234567","cancellation"));if code!=201{t.Fatal(code,string(body))}
 var first models.PaymentRefund;json.Unmarshal(body,&first)
 if strings.Contains(string(body),"256771234567"){t.Fatal("full payer number exposed")}
 if code,_:=request("POST","/admin/refunds",makeDraft(1000,"0771234567","cancellation"));code!=409{t.Fatal("over-refund reservation accepted",code)}
 if code,_:=request("POST","/admin/refunds/"+first.ID+"/submit",map[string]bool{"confirm":false});code!=400{t.Fatal("missing confirmation accepted",code)}
 code,body=request("POST","/admin/refunds/"+first.ID+"/submit",map[string]bool{"confirm":true});if code!=200{t.Fatal(code,string(body))}
 if err:=tx.First(&first,"id = ?",first.ID).Error;err!=nil{t.Fatal(err)}
 if first.Status!="succeeded" || first.ProviderReference==""{t.Fatal("refund not independently verified",first.Status)}
 request("POST","/admin/refunds/"+first.ID+"/submit",map[string]bool{"confirm":true})
 if posts.Load()!=1{t.Fatal("duplicate payout",posts.Load())}
 if err:=tx.First(&order,"id = ?",order.ID).Error;err!=nil{t.Fatal(err)}
 if order.Status!="cancelled" || order.PaymentStatus!="paid"{t.Fatal("original payment/history corrupted")}
 flow,err:=services.BuildRefundCashflow(tx,services.ReconciliationFilter{});if err!=nil || flow.Collections!=2000 || flow.RefundsPaid!=1200 || flow.NetCollections!=800{t.Fatal(flow,err)}
 code,body=request("POST","/admin/refunds",makeDraft(800,"0771234567","cancellation"));if code!=201{t.Fatal(code,string(body))}
 var second models.PaymentRefund;json.Unmarshal(body,&second)
 tx.Create(&models.OrderStatusEvent{OrderID:order.ID,Status:"delivered",Note:"Historic delivered event"})
 if _,err:=claimRefund(tx,second.ID,userID);err==nil{t.Fatal("historic delivery bypassed cancellation guard")}
 if code,_:=request("POST","/admin/refunds/"+second.ID+"/reject",nil);code!=200{t.Fatal(code)}
 code,body=request("POST","/admin/refunds",makeDraft(800,"0771234567","return_settlement"));if code!=201{t.Fatal(code,string(body))}
 var settlement models.PaymentRefund;json.Unmarshal(body,&settlement)
 // A crash after the claim leaves a reservation and cannot send twice.
 claimed,err:=claimRefund(tx,settlement.ID,userID);if err!=nil{t.Fatal(err)}
 if claimed.Status!="submitting"{t.Fatal(claimed.Status)}
 if _,err=claimRefund(tx,settlement.ID,userID);err==nil{t.Fatal("claimed refund can be resent")}
 if _,err=refundableAmount(tx,attempt);err!=nil{t.Fatal(err)}
 _=context.Background()
}
