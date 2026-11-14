from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def put(root,name,text):
 p=root/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text,encoding='utf-8')
put(api,'routes/release_database_test.go','''package routes
import(
 "encoding/json";"net/http/httptest";"strings";"testing";"os";"time";"fmt"
 "afrigoals-api.com/models";"afrigoals-api.com/services"
 "github.com/google/uuid";"github.com/joho/godotenv";"github.com/gofiber/fiber/v2"
 "gorm.io/driver/postgres";"gorm.io/gorm";"gorm.io/gorm/logger";"gorm.io/datatypes"
)
func TestReleaseDatabaseFlows(t *testing.T){
 if os.Getenv("TEST_RELEASE_DB")!="true"{t.Skip("opt-in isolated development database test")}
 _=godotenv.Load("../.env");cfg:=services.LoadConfig()
 database,err:=gorm.Open(postgres.New(postgres.Config{DSN:cfg.DBPostgresDSN,PreferSimpleProtocol:true}),&gorm.Config{Logger:logger.Default.LogMode(logger.Silent)});if err!=nil{t.Fatal("could not connect to development database")}
 tx:=database.Begin();if tx.Error!=nil{t.Fatal(tx.Error)};defer tx.Rollback()
 schema:="release_test_"+strings.ReplaceAll(uuid.NewString(),"-","")
 if err:=tx.Exec("CREATE SCHEMA "+schema).Error;err!=nil{t.Fatal(err)}
 if err:=tx.Exec("SET LOCAL search_path TO "+schema+", public").Error;err!=nil{t.Fatal(err)}
 if err:=tx.AutoMigrate(&models.User{},&models.Product{},&models.Order{},&models.OrderItem{},&models.OrderStatusEvent{},&models.Review{},&models.Rider{},&models.OrderDeliveryAssignment{},&models.PasswordResetChallenge{},&models.DeliverySettings{});err!=nil{t.Fatal(err)}
 hash,_:=services.HashPassword("original-password-123")
 owner:=models.User{Email:"owner@release.test",Name:"Owner",Role:"user",PasswordHash:hash,EmailVerified:true};other:=models.User{Email:"other@release.test",Role:"user",PasswordHash:hash,EmailVerified:true};vendor:=models.User{Email:"vendor@release.test",Role:"vendor",Vendor:"Vendor A",PasswordHash:hash,EmailVerified:true}
 for _,u:=range []*models.User{&owner,&other,&vendor}{if err:=tx.Create(u).Error;err!=nil{t.Fatal(err)}}
 order:=models.Order{UserID:owner.ID,Email:owner.Email,OrderNumber:"RELEASE-1",Status:"confirmed",PaymentStatus:"paid",PaymentMethod:"mobile_money",Currency:"UGX",Subtotal:1100,DeliveryFee:300,Total:1400}
 if err:=tx.Create(&order).Error;err!=nil{t.Fatal(err)}
 current,role:=owner.ID,"user"
 auth:=func(c *fiber.Ctx)error{c.Locals("userID",current);c.Locals("role",role);return c.Next()}
 app:=fiber.New();deps:=Deps{DB:tx,Config:cfg,JWT:services.NewJWT(cfg)};RegisterOrders(app,auth,deps);RegisterAuth(app,deps);RegisterVendors(app,auth,deps)
 request:=func(method,path,body string)(int,string){req:=httptest.NewRequest(method,path,strings.NewReader(body));req.Header.Set("Content-Type","application/json");res,err:=app.Test(req,10000);if err!=nil{t.Fatal(err)};defer res.Body.Close();var data any;if err:=json.NewDecoder(res.Body).Decode(&data);err!=nil{t.Fatal(err)};encoded,_:=json.Marshal(data);return res.StatusCode,string(encoded)}
 t.Run("order privacy",func(t *testing.T){
  if status,_:=request("GET","/orders/"+order.ID,"");status!=200{t.Fatal(status)}
  current=other.ID;if status,body:=request("GET","/orders/"+order.ID,"");status!=404||strings.Contains(body,owner.Email){t.Fatalf("cross-account exposure: %d %s",status,body)}
  role="admin";if status,_:=request("GET","/orders/"+order.ID,"");status!=200{t.Fatal(status)};role="user";current=owner.ID
 })
 t.Run("single use password reset",func(t *testing.T){
  challenge:=models.PasswordResetChallenge{ID:uuid.NewString(),UserID:owner.ID,ExpiresAt:time.Now().Add(time.Hour)};challenge.CodeHash=services.HashLoginCode(challenge.ID,"123456",cfg.TwoFAPepper);if err:=tx.Create(&challenge).Error;err!=nil{t.Fatal(err)}
  body:=fmt.Sprintf(`{"challengeId":%q,"code":"123456","newPassword":"replacement-password-123"}`,challenge.ID)
  if status,result:=request("POST","/auth/reset-password/confirm",body);status!=200{t.Fatalf("%d %s",status,result)}
  if status,_:=request("POST","/auth/reset-password/confirm",body);status!=401{t.Fatal("reset code reused",status)}
  var updated models.User;if err:=tx.First(&updated,"id = ?",owner.ID).Error;err!=nil{t.Fatal(err)}
  if updated.PasswordHash==hash{t.Fatal("password did not change")}
 })
 t.Run("vendor data isolation",func(t *testing.T){
  a:=models.Product{Name:"A product",Slug:"release-a",Vendor:"Vendor A",Price:200,Currency:"UGX",Stock:10,Images:datatypes.JSON("[]"),Variants:datatypes.JSON("[]")};b:=models.Product{Name:"secret Vendor B product",Slug:"release-b",Vendor:"Vendor B",Price:900,Currency:"UGX",Stock:10,Images:datatypes.JSON("[]"),Variants:datatypes.JSON("[]")}
  if err:=tx.Create(&a).Error;err!=nil{t.Fatal(err)};if err:=tx.Create(&b).Error;err!=nil{t.Fatal(err)}
  for _,line:=range []models.OrderItem{{OrderID:order.ID,ProductID:a.ID,ProductName:a.Name,VendorName:a.Vendor,Quantity:1,PriceAtPurchase:200,LineSubtotal:200},{OrderID:order.ID,ProductID:b.ID,ProductName:b.Name,VendorName:b.Vendor,Quantity:1,PriceAtPurchase:900,LineSubtotal:900}}{if err:=tx.Create(&line).Error;err!=nil{t.Fatal(err)}}
  current=vendor.ID;role="vendor";status,body:=request("GET","/vendor/overview?vendor=Vendor%20B","")
  if status!=200||!strings.Contains(body,`"sales":200`)||strings.Contains(body,"secret Vendor B")||strings.Contains(body,owner.Email){t.Fatalf("vendor scope failed: %d %s",status,body)}
  current=owner.ID;role="user";if status,_:=request("GET","/vendor/overview","");status!=403{t.Fatal(status)}
 })
 t.Run("rider payout snapshots",func(t *testing.T){
  rider:=models.Rider{Name:"Release rider",Email:"rider@release.test",Phone:"256771234567",PasswordHash:hash,NationalIDNumber:"test-only",Status:models.RiderStatusApproved};if err:=tx.Create(&rider).Error;err!=nil{t.Fatal(err)}
  settings:=services.DefaultDeliverySettings();rate:=.15;settings.ServiceFeeRate=&rate;if err:=services.SaveDeliverySettings(tx,settings,"test");err!=nil{t.Fatal(err)}
  service:=services.NewRiderService(tx,nil);assignment,err:=service.AssignOrder(order.ID,rider.ID);if err!=nil{t.Fatal(err)}
  if assignment.ServiceFee!=45||assignment.RiderPayout!=255||assignment.Order.RiderPayout!=255{t.Fatal("wrong rider payout",assignment)}
  accepted,err:=service.AcceptOrder(rider.ID,order.ID);if err!=nil||accepted.Order.RiderPayout!=255{t.Fatal("accept lost payout",err)}
  rate=.5;settings.ServiceFeeRate=&rate;if err:=services.SaveDeliverySettings(tx,settings,"test");err!=nil{t.Fatal(err)}
  again,err:=service.AssignOrder(order.ID,rider.ID);if err!=nil||again.RiderPayout!=255||again.AcceptedAt==nil{t.Fatal("repeat assignment repriced or reset accepted delivery",err)}
 })
}
''')
put(web,'tests/admin-insights.test.ts','''import "./setup-server-only";
import assert from "node:assert/strict";import{afterEach,mock,test}from"node:test";
import{GET}from"../app/api/admin/insights/route";import{json,mockFetch,setRequestCookies}from"./helpers";import{AUTH_COOKIE_NAME}from"../lib/auth/constants";
afterEach(()=>mock.restoreAll());
test("dashboard counts only collected non-cancelled payments as revenue",async()=>{
 setRequestCookies({[AUTH_COOKIE_NAME]:"admin-token"});
 mockFetch(()=>json([
 {id:"paid",status:"confirmed",paymentStatus:"paid",paymentMethod:"mobile_money",currency:"UGX",subtotal:500,deliveryFee:300,total:800,items:[{productId:"p1",productName:"Kit",quantity:1,lineSubtotal:500}]},
 {id:"pending",status:"confirmed",paymentStatus:"unpaid",paymentMethod:"cod",subtotal:500,deliveryFee:300,total:800,items:[]},
 {id:"cancelled",status:"cancelled",paymentStatus:"paid",subtotal:500,deliveryFee:300,total:800,items:[]}
 ]));
 const response=await GET();const report=await response.json();assert.equal(report.totals.revenue,800);assert.equal(report.totals.paidOrders,1);assert.equal(report.totals.outstandingCod,800);assert.equal(report.totals.productRevenue+report.totals.deliveryFees,report.totals.revenue);assert.equal(report.topProducts[0].units,1);
});
''')
import json
p=web/'package.json';data=json.loads(p.read_text(encoding='utf-8'));data['scripts']['test']+=' tests/admin-insights.test.ts';put(web,'package.json',json.dumps(data,indent=2)+'\n')
print('Added isolated-schema database regression checks and admin revenue tests.')
