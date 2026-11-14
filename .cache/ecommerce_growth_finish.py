from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def put(root,name,text):
 p=root/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text,encoding='utf-8')
# Explicit empty arrays keep new dashboards usable before any data is entered.
for name in ['routes/commerce_growth.go','routes/vendor.go','routes/email_campaigns.go']:
 p=api/name;s=p.read_text(encoding='utf-8')
 for variable,typ in [('stores','PartnerStore'),('users','User'),('campaigns','EmailCampaign'),('deliveries','EmailDelivery')]:
  s=s.replace(f'var {variable} []models.{typ}',f'{variable} := make([]models.{typ}, 0)')
 put(api,name,s)
p=api/'services/riders.go';s=p.read_text(encoding='utf-8').replace('var riders []models.Rider','riders := make([]models.Rider,0)',1)
s=s.replace('var assignment models.OrderDeliveryAssignment\n\terr := s.DB.Transaction', 'var assignment models.OrderDeliveryAssignment\n changed := false\n\terr := s.DB.Transaction',1)
s=s.replace('assignment = models.OrderDeliveryAssignment{','changed = true\n assignment = models.OrderDeliveryAssignment{',1)
s=s.replace('assignment.RiderID = riderID','changed = true\n assignment.RiderID = riderID',1)
s=s.replace('assignment.Order.DeliveryServiceFee = assignment.ServiceFee','if err==nil && changed && s.Email!=nil { _=s.Email.SendRiderOrderAssigned(assignment.Rider.Email,assignment.Rider.Name,assignment.Order.OrderNumber) }\n assignment.Order.DeliveryServiceFee = assignment.ServiceFee',1)
put(api,'services/riders.go',s)
p=api/'services/email_campaigns.go';s=p.read_text(encoding='utf-8');lines=s.splitlines()
for i,line in enumerate(lines):
 if 'body := ' in line:
  lines[i]='\t\t\tbody := "<h1>" + html.EscapeString(campaign.Subject) + "</h1><p>" + strings.ReplaceAll(html.EscapeString(campaign.Body), "\\n", "<br>") + `</p><p><a href="` + html.EscapeString(unsubscribe) + `">Unsubscribe from marketing emails</a></p>`'
 if 'err := sender.sendHTMLWithKey' in line:
  lines[i]='\t\t\ttime.Sleep(650 * time.Millisecond)\n'+line
put(api,'services/email_campaigns.go','\n'.join(lines)+'\n')
p=api/'routes/email_campaigns.go';s=p.read_text(encoding='utf-8');lines=s.splitlines()
for i,line in enumerate(lines):
 if 'SendString("<h1>AfriGoals email preferences' in line:
  lines[i]='\t\treturn c.Type("html").SendString(`<h1>AfriGoals email preferences</h1><p>Stop marketing emails. Order and security updates are unaffected.</p><form method="post"><button type="submit">Unsubscribe</button></form>`)'
put(api,'routes/email_campaigns.go','\n'.join(lines)+'\n')
p=api/'services/commerce_reports.go';s=p.read_text(encoding='utf-8').replace('Where("role IN ?", []string{"user", "vendor"})','Where("role IN ? AND email_verified = ?", []string{"user", "vendor"}, true)');put(api,'services/commerce_reports.go',s)

# Reconciliation exposes truncation and current commission, while retaining saved payouts.
p=api/'services/reconciliation.go';s=p.read_text(encoding='utf-8').replace('type ReconciliationReport struct {','type ReconciliationReport struct {\n OrdersTruncated bool `json:"ordersTruncated"`',1)
s=s.replace('report := ReconciliationReport{','settings,err:=LoadDeliverySettings(db)\n if err!=nil{return ReconciliationReport{},err}\n rate:=deliveryServiceFeeRate();if settings.ServiceFeeRate!=nil{rate=*settings.ServiceFeeRate}\n report := ReconciliationReport{',1).replace('ServiceFeeRate: deliveryServiceFeeRate(),','ServiceFeeRate: rate,\n OrdersTruncated: len(paid)>maxRegisterOrders,',1);put(api,'services/reconciliation.go',s)
p=web/'app/(admin)/admin/reconciliation/page.tsx';s=p.read_text(encoding='utf-8').replace('type Report = {','type Report = {\n ordersTruncated?:boolean;',1).replace('Paid order register\n','Paid order register\n',1)
needle='''            Paid order register
          </h2>'''
s=s.replace(needle,needle+'''{report?.ordersTruncated&&<p className="mt-2 text-sm text-amber-700">Showing the latest 200 paid orders. Totals include the full selected period; narrow the date range to inspect older orders.</p>}''',1);put(web,'app/(admin)/admin/reconciliation/page.tsx',s)

# Public customer entry points for the newly added directories/dashboards.
p=web/'components/app/Header.tsx';s=p.read_text(encoding='utf-8').replace('const navLinks = [','const navLinks = [\n {label:"Partner stores",href:"/partner-stores"},',1);put(web,'components/app/Header.tsx',s)
put(web,'app/api/admin/orders/[id]/assign-rider/route.ts','''import{NextResponse}from"next/server";import{authedFetch}from"@/lib/api/proxy";
export async function POST(request:Request,{params}:{params:Promise<{id:string}>}){const{id}=await params;const response=await authedFetch(`/admin/orders/${encodeURIComponent(id)}/assign-rider`,{method:"POST",headers:{"Content-Type":"application/json"},body:await request.text()});return new NextResponse(await response.text(),{status:response.status,headers:{"Content-Type":"application/json"}})}
''')
p=web/'app/(admin)/admin/riders/page.tsx';s=p.read_text(encoding='utf-8')
s=s.replace('const [riders, setRiders]', 'const [orderId,setOrderId]=useState("");\n const [riderId,setRiderId]=useState("");\n const [riders, setRiders]',1)
marker='  return ('
index=s.index(marker)
s=s[:index]+''' async function assign(event:React.FormEvent){event.preventDefault();setBusy(true);setError("");try{const response=await fetch(`/api/admin/orders/${encodeURIComponent(orderId)}/assign-rider`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({riderId})});const body=await response.json();if(!response.ok)throw Error(body.error);setError("Delivery assigned. The rider can view it in /rider.")}catch(cause){setError(cause instanceof Error?cause.message:"Could not assign delivery")}finally{setBusy(false)}}
'''+s[index:]
marker='<h1 className="text-2xl font-semibold">Rider accounts</h1>'
s=s.replace(marker,marker+'''<form onSubmit={assign} className="space-y-3 rounded border p-4"><h2 className="font-semibold">Assign a delivery</h2><label className="block" htmlFor="assign-order">Order ID (from its admin order page)<input id="assign-order" required className="block w-full rounded border p-2" value={orderId} onChange={event=>setOrderId(event.target.value)}/></label><label className="block" htmlFor="assign-rider">Approved rider<select id="assign-rider" required className="block w-full rounded border p-2" value={riderId} onChange={event=>setRiderId(event.target.value)}><option value="">Select a rider</option>{riders.filter(r=>r.status==="approved").map(r=><option value={r.id} key={r.id}>{r.name} · {r.email}</option>)}</select></label><Button disabled={busy}>Assign delivery</Button></form>''',1);put(web,'app/(admin)/admin/riders/page.tsx',s)
print('Finished empty states, campaign HTML, rider assignment UI and reconciliation transparency.')
