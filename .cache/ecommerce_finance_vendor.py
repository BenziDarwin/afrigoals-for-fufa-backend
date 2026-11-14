from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def put(root,name,text):
 p=root/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text,encoding='utf-8')
def replace(root,name,old,new):
 p=root/name;s=p.read_text(encoding='utf-8');assert old in s,(name,old[:50]);put(root,name,s.replace(old,new,1))

replace(api,'models/user.go','type User struct {','type User struct {\n Vendor string `json:"vendor,omitempty" gorm:"index"`')
replace(api,'models/order.go','type Order struct {','type Order struct {\n DeliveryStatus string `json:"deliveryStatus,omitempty" gorm:"-"`')
replace(api,'services/delivery_settings.go','type DeliverySettings struct {','type DeliverySettings struct {\n ServiceFeeRate *float64 `json:"serviceFeeRate"`')
replace(api,'services/delivery_settings.go','return DeliverySettings{Mode: mode,','rate := deliveryServiceFeeRate()\n return DeliverySettings{ServiceFeeRate: &rate, Mode: mode,')
replace(api,'services/delivery_settings.go','p := s.Pricing','''if s.ServiceFeeRate != nil && (math.IsNaN(*s.ServiceFeeRate) || math.IsInf(*s.ServiceFeeRate,0) || *s.ServiceFeeRate < 0 || *s.ServiceFeeRate >= 1) { return errors.New("commission must be between 0% and less than 100%") }
 p := s.Pricing''')
replace(api,'services/delivery_settings.go','return settings, ValidateDeliverySettings(settings)','''if settings.ServiceFeeRate == nil { rate:=deliveryServiceFeeRate();settings.ServiceFeeRate=&rate }
 return settings, ValidateDeliverySettings(settings)''')

p=api/'services/riders.go';s=p.read_text(encoding='utf-8').replace('"gorm.io/gorm"','"gorm.io/gorm"\n"gorm.io/gorm/clause"',1)
start=s.index('func (s *RiderService) AssignOrder(');end=s.index('\nfunc (s *RiderService) ListAssignedOrders',start)
s=s[:start]+'''func (s *RiderService) AssignOrder(orderID string, riderID string) (models.OrderDeliveryAssignment, error) {
 var assignment models.OrderDeliveryAssignment
 err:=s.DB.Transaction(func(tx *gorm.DB)error{
  var rider models.Rider
  if err:=tx.First(&rider,"id = ? AND status = ?",riderID,models.RiderStatusApproved).Error;err!=nil{return errors.New("approved rider not found")}
  var order models.Order
  if err:=tx.Clauses(clause.Locking{Strength:"UPDATE"}).First(&order,"id = ?",orderID).Error;err!=nil{return err}
  if order.Status=="cancelled"||order.Status=="canceled"||order.Status=="delivered"{return errors.New("completed or cancelled orders cannot be assigned")}
  if order.PaymentMethod=="mobile_money"&&order.PaymentStatus!="paid"{return errors.New("confirm online payment before assigning delivery")}
  err:=tx.Where("order_id = ?",orderID).First(&assignment).Error
  if err!=nil&&!errors.Is(err,gorm.ErrRecordNotFound){return err}
  if assignment.ID!=""&&assignment.RiderID==riderID{return nil}
  if assignment.ID==""{
   settings,err:=LoadDeliverySettings(tx);if err!=nil{return err}
   rate:=deliveryServiceFeeRate();if settings.ServiceFeeRate!=nil{rate=*settings.ServiceFeeRate}
   fee,payout:=calculateDeliveryPayoutAtRate(order.DeliveryFee,rate)
   assignment=models.OrderDeliveryAssignment{OrderID:orderID,RiderID:riderID,Status:models.DeliveryStatusAssigned,DeliveryFee:order.DeliveryFee,ServiceFee:fee,RiderPayout:payout}
   return tx.Create(&assignment).Error
  }
  // Reassign the delivery without repricing its historical commission.
  assignment.RiderID=riderID;assignment.Status=models.DeliveryStatusAssigned;assignment.AcceptedAt=nil
  return tx.Save(&assignment).Error
 })
 if err!=nil{return assignment,err}
 err=s.DB.Preload("Order").Preload("Rider").First(&assignment,"id = ?",assignment.ID).Error
 assignment.Order.DeliveryServiceFee=assignment.ServiceFee;assignment.Order.RiderPayout=assignment.RiderPayout;assignment.Order.DeliveryStatus=assignment.Status
 return assignment,err
}
''' + s[end:]
s=s.replace('order.RiderPayout = assignment.RiderPayout','order.RiderPayout = assignment.RiderPayout\n order.DeliveryStatus = assignment.Status',1)
s=s.replace('''func calculateDeliveryPayout(deliveryFee float64) (serviceFee float64, riderPayout float64) {
''','''func calculateDeliveryPayout(deliveryFee float64) (serviceFee float64, riderPayout float64) {
 return calculateDeliveryPayoutAtRate(deliveryFee,deliveryServiceFeeRate())
}
func calculateDeliveryPayoutAtRate(deliveryFee,rate float64) (serviceFee float64, riderPayout float64) {
''',1).replace('deliveryFee * deliveryServiceFeeRate()','deliveryFee * rate',1)
start=s.index('func (s *RiderService) AcceptOrder(');block=s[start:]
block=block.replace('\treturn assignment, nil',''' assignment.Order.DeliveryServiceFee=assignment.ServiceFee
 assignment.Order.RiderPayout=assignment.RiderPayout
 assignment.Order.DeliveryStatus=assignment.Status
 return assignment, nil''',1);s=s[:start]+block
put(api,'services/riders.go',s)

put(api,'routes/vendor.go','''package routes
import(
 "strings"
 "afrigoals-api.com/models"
 "afrigoals-api.com/middleware"
 "github.com/gofiber/fiber/v2"
)
type vendorProduct struct{
 ProductID string `json:"productId"`; Name string `json:"name"`; Stock int64 `json:"stock"`
 Units int64 `json:"units"`; Sales float64 `json:"sales"`; ReviewCount int64 `json:"reviewCount"`; AverageRating float64 `json:"averageRating"`
}
func RegisterVendors(router fiber.Router,auth fiber.Handler,deps Deps){
 router.Get("/admin/vendors",auth,middleware.RequireAdmin(),func(c *fiber.Ctx)error{
  var users []models.User
  if err:=deps.DB.Where("role = ?","vendor").Order("email").Find(&users).Error;err!=nil{return c.Status(500).JSON(fiber.Map{"error":"could not load vendor accounts"})}
  return c.JSON(users)
 })
 router.Post("/admin/vendors",auth,middleware.RequireAdmin(),func(c *fiber.Ctx)error{
  var input struct{Email string `json:"email"`;Vendor string `json:"vendor"`}
  if c.BodyParser(&input)!=nil||strings.TrimSpace(input.Email)==""||strings.TrimSpace(input.Vendor)==""{return c.Status(400).JSON(fiber.Map{"error":"email and vendor are required"})}
  var user models.User
  if deps.DB.Where("email = ?",strings.ToLower(strings.TrimSpace(input.Email))).First(&user).Error!=nil{return c.Status(404).JSON(fiber.Map{"error":"customer account not found; ask the vendor to register first"})}
  if isAdminRole(user.Role){return c.Status(400).JSON(fiber.Map{"error":"admin accounts cannot be reassigned to vendors"})}
  if !user.EmailVerified{return c.Status(400).JSON(fiber.Map{"error":"vendor must verify their account email first"})}
  if err:=deps.DB.Model(&user).Updates(map[string]any{"role":"vendor","vendor":models.NormalizeVendor(input.Vendor)}).Error;err!=nil{return c.Status(500).JSON(fiber.Map{"error":"could not assign vendor account"})}
  return c.JSON(fiber.Map{"success":true})
 })
 router.Get("/vendor/overview",auth,func(c *fiber.Ctx)error{
  userID,_:=c.Locals("userID").(string)
  var user models.User
  if userID==""||deps.DB.First(&user,"id = ?",userID).Error!=nil||user.Role!="vendor"||user.Vendor==""{return c.Status(403).JSON(fiber.Map{"error":"vendor account required"})}
  // The vendor scope comes only from the authenticated database account.
  var products []models.Product
  if err:=deps.DB.Where("LOWER(vendor) = LOWER(?)",user.Vendor).Find(&products).Error;err!=nil{return c.Status(500).JSON(fiber.Map{"error":"could not load vendor products"})}
  report:=make([]vendorProduct,0,len(products));var units int64;var sales float64
  for _,p:=range products{
   row:=vendorProduct{ProductID:p.ID,Name:p.Name,Stock:p.Stock}
   if err:=deps.DB.Table("order_items AS i").Select("COALESCE(SUM(i.quantity),0) AS units, COALESCE(SUM(CASE WHEN i.line_subtotal > 0 THEN i.line_subtotal ELSE i.price_at_purchase*i.quantity END),0) AS sales").Joins("JOIN orders o ON o.id = i.order_id").Where("i.product_id = ? AND LOWER(i.vendor_name) = LOWER(?) AND o.payment_status = ? AND LOWER(o.status) NOT IN ?",p.ID,user.Vendor,"paid",[]string{"cancelled","canceled"}).Scan(&row).Error;err!=nil{return c.Status(500).JSON(fiber.Map{"error":"could not calculate vendor sales"})}
   if err:=deps.DB.Model(&models.Review{}).Select("COUNT(*) AS review_count, COALESCE(AVG(rating),0) AS average_rating").Where("product_id = ? AND approved = ?",p.ID,true).Scan(&row).Error;err!=nil{return c.Status(500).JSON(fiber.Map{"error":"could not load reviews"})}
   units+=row.Units;sales+=row.Sales;report=append(report,row)
  }
  return c.JSON(fiber.Map{"vendor":user.Vendor,"currency":"UGX","units":units,"sales":sales,"products":report})
 })
}
''')
replace(api,'routes/routes.go','RegisterDeliveryPricing(api, auth, deps)','RegisterVendors(api, auth, deps)\n RegisterDeliveryPricing(api, auth, deps)')
replace(web,'lib/auth/client.ts','role: "admin" | "user";','role: "admin" | "user" | "vendor";')
replace(web,'lib/auth/client.ts','role !== "admin" && role !== "user"','role !== "admin" && role !== "user" && role !== "vendor"')
replace(web,'app/(app)/signin/page.tsx','''    } else {
      router.replace(next);
''','''    } else if (data.user.role === "vendor") {
      router.replace("/vendor");
    } else {
      router.replace(next);
''')
replace(web,'app/(admin)/admin/layout.tsx','const navItems = [','const navItems = [\n {label:"Vendor accounts",href:"/admin/vendors",icon:Package},')
replace(web,'app/(admin)/admin/delivery-pricing/page.tsx','type Settings = {','type Settings = {\n serviceFeeRate: number;')
replace(web,'app/(admin)/admin/delivery-pricing/page.tsx','<h2 className="font-semibold">Distance-based rates</h2>', '''<label htmlFor="commission" className="block text-sm font-medium">Afrigoals delivery commission (%)
 <input id="commission" className={inputClass} type="number" required min="0" max="99.99" step="0.01" value={(settings.serviceFeeRate??0.15)*100} onChange={event=>setSettings({...settings,serviceFeeRate:event.target.valueAsNumber/100})}/>
 </label><p className="my-2 text-sm text-zinc-500">Rider payout is the delivery fee minus this commission. Saved rates apply to new assignments; existing payouts stay unchanged.</p>
 <h2 className="font-semibold">Distance-based rates</h2>''')

for route,backend,methods in [('admin/vendors','/admin/vendors',['GET','POST']),('vendor/overview','/vendor/overview',['GET'])]:
 text='import { NextResponse } from "next/server";\nimport { authedFetch } from "@/lib/api/proxy";\n'
 for method in methods:
  text+=f'''export async function {method}({'request:Request' if method=='POST' else ''}){{
   const response=await authedFetch("{backend}"{', {method:"POST",headers:{"Content-Type":"application/json"},body:await request.text()}' if method=='POST' else ''});
   return new NextResponse(await response.text(),{{status:response.status,headers:{{"Content-Type":"application/json","Cache-Control":"no-store"}}}});
  }}\n'''
 put(web,f'app/api/{route}/route.ts',text)
put(web,'app/(app)/vendor/page.tsx','''"use client";
import{useEffect,useState}from"react";
import{formatPrice}from"@/lib/utils";
type Report={vendor:string;sales:number;units:number;products:{productId:string;name:string;stock:number;units:number;sales:number;reviewCount:number;averageRating:number}[]};
export default function VendorPage(){
 const[report,setReport]=useState<Report|null>(null);const[error,setError]=useState("");
 useEffect(()=>{fetch("/api/vendor/overview",{cache:"no-store"}).then(async r=>{const data=await r.json();if(!r.ok)throw Error(data.error);setReport(data)}).catch(e=>setError(e.message))},[]);
 return <main className="mx-auto max-w-5xl space-y-6 p-6"><h1 className="text-2xl font-semibold">Vendor performance</h1>{error&&<p role="alert">{error}</p>}{!report&&!error&&<p>Loading your products…</p>}{report&&<><h2>{report.vendor}</h2><p>{report.units} units sold · {formatPrice(report.sales)} paid product sales</p><div className="overflow-x-auto"><table className="w-full text-left"><thead><tr>{["Product","Stock","Units sold","Paid sales","Reviews"].map(label=><th className="p-3" key={label}>{label}</th>)}</tr></thead><tbody>{report.products.map(p=><tr key={p.productId} className="border-t"><td className="p-3">{p.name}</td><td>{p.stock}</td><td>{p.units}</td><td>{formatPrice(p.sales)}</td><td>{p.reviewCount?`${p.averageRating.toFixed(1)} / 5 (${p.reviewCount})`:"No reviews"}</td></tr>)}</tbody></table></div></>}</main>
}
''')
put(web,'app/(admin)/admin/vendors/page.tsx','''"use client";
import{useEffect,useState}from"react";
import{Button}from"@/components/ui/button";
export default function VendorsPage(){
 const[accounts,setAccounts]=useState<{id:string;email:string;vendor:string}[]>([]);const[email,setEmail]=useState("");const[vendor,setVendor]=useState("");const[message,setMessage]=useState("");const[busy,setBusy]=useState(false);
 async function load(){const r=await fetch("/api/admin/vendors",{cache:"no-store"});const data=await r.json();if(!r.ok)throw Error(data.error);setAccounts(data)}
 useEffect(()=>{load().catch(e=>setMessage(e.message))},[]);
 async function save(e:React.FormEvent){e.preventDefault();setBusy(true);try{const r=await fetch("/api/admin/vendors",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({email,vendor})});const data=await r.json();if(!r.ok)throw Error(data.error);setMessage("Vendor assigned. The account can sign in and visit /vendor.");await load()}catch(e){setMessage(e instanceof Error?e.message:"Could not save")}finally{setBusy(false)}}
 return <section className="max-w-3xl space-y-6 p-6"><h1 className="text-2xl font-semibold">Vendor accounts</h1><p>Assign an existing verified account to the vendor name used on its products. Each account can view only that vendor's performance.</p><form onSubmit={save} className="space-y-4"><label className="block" htmlFor="vendor-email">Account email<input id="vendor-email" required type="email" className="block w-full rounded border p-2" value={email} onChange={e=>setEmail(e.target.value)}/></label><label className="block" htmlFor="vendor-name">Vendor name<input id="vendor-name" required className="block w-full rounded border p-2" value={vendor} onChange={e=>setVendor(e.target.value)}/></label><Button disabled={busy}>{busy?"Saving…":"Assign vendor account"}</Button></form>{message&&<p role="status">{message}</p>}<ul>{accounts.map(a=><li key={a.id} className="border-t py-3">{a.email} · {a.vendor}</li>)}</ul></section>
}
''')
print('Added vendor-scoped accounts and reports; fixed rider payout snapshots and admin commission settings.')
