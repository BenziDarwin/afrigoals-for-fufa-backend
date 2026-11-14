from pathlib import Path
import shutil
src=Path('F:/Afrigoals/Afrigoals-API/.cache/customer-refunds');api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def edit(p,a,b):
 s=p.read_text(encoding='utf-8');assert a in s,(str(p),a[:80]);p.write_text(s.replace(a,b),encoding='utf-8')
for name,target in {
 'refund_request.go':api/'models/refund_request.go',
 'refund_requests.go':api/'routes/refund_requests.go',
 'CustomerRefundRequests.tsx':web/'components/orders/CustomerRefundRequests.tsx',
 'AdminRefundRequests.tsx':web/'components/admin/AdminRefundRequests.tsx',
 'refund-requests.ts':web/'lib/orders/refund-requests.ts',
 'customer_route.ts':web/'app/api/orders/[id]/refund-requests/route.ts',
 'admin_route.ts':web/'app/api/admin/refund-requests/route.ts',
 'admin_action_route.ts':web/'app/api/admin/refund-requests/[id]/[action]/route.ts',
}.items():
 target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(src/name,target)
edit(api/'models/payment_refund.go','type PaymentRefund struct {','type PaymentRefund struct {\n RefundRequestID *string `json:"refundRequestId,omitempty" gorm:"uniqueIndex"`')
edit(api/'db/db.go','&models.PaymentRefund{},','&models.PaymentRefund{},\n &models.RefundRequest{},')
for name in ['routes/release_database_test.go','routes/refund_database_test.go']:
 edit(api/name,'&models.PaymentRefund{},','&models.PaymentRefund{}, &models.RefundRequest{},') if '&models.PaymentRefund{},' in (api/name).read_text() else edit(api/name,'&models.PaymentRefund{}','&models.PaymentRefund{}, &models.RefundRequest{}')
edit(api/'routes/routes.go','RegisterRefunds(api, auth, deps)','RegisterRefunds(api, auth, deps)\n RegisterRefundRequests(api, auth, deps)')
# Avoid a database query per item in the admin review queue.
p=api/'routes/refund_requests.go';s=p.read_text();a=s.index('  rows:=make([]adminRefundRequestView');b=s.index('  return c.JSON(rows)',a)
s=s[:a]+'''  rows:=make([]adminRefundRequestView,0,len(views))
  orderIDs:=[]string{};for _,view:=range views{orderIDs=append(orderIDs,view.OrderID)}
  orders:=[]models.Order{}
  if len(orderIDs)>0{if err:=deps.DB.Select("id","status").Where("id IN ?",orderIDs).Find(&orders).Error;err!=nil{return refundError(c,500,"Could not load order statuses")}}
  statuses:=map[string]string{};for _,order:=range orders{statuses[order.ID]=order.Status}
  for _,view:=range views{rows=append(rows,adminRefundRequestView{refundRequestView:view,Phone:view.Phone,OrderStatus:statuses[view.OrderID]})}
'''+s[b:];p.write_text(s,encoding='utf-8')
p=web/'app/(app)/orders/[id]/page.tsx'
edit(p,'import { OrderDeliveryContactEditor }','import { CustomerRefundRequests } from "@/components/orders/CustomerRefundRequests";\nimport { OrderDeliveryContactEditor }')
edit(p,'        <OrderProgressTracker status={order.status} />','        <OrderProgressTracker status={order.status} />\n        <CustomerRefundRequests orderId={order.id} />')
p=web/'app/(admin)/admin/refunds/page.tsx'
edit(p,'import Link from "next/link";','import Link from "next/link";\nimport { AdminRefundRequests } from "@/components/admin/AdminRefundRequests";')
edit(p,'      <form onSubmit={prepare}','      <AdminRefundRequests<Refund> onApproved={(refund) => { setReview(refund); setConfirmed(false); void load(); }} />\n      <form onSubmit={prepare}')
with (api/'docs/refunds.md').open('a',encoding='utf-8') as f:f.write('''

## Customer requests

My Orders > Order details > Refunds and returns lets the order owner choose a verified payment and submit their name, original payer number, requested amount, reason, product/return details and optional mobile money receipt reference. The order and email come from the authenticated account, never from supplied customer IDs. Requests are records for review; they do not cancel an order, reserve payout funds or initiate money transfers. One active request is allowed per order. Customers can see their own request, the administrator's public response, approved amount and verified payout transaction/status. Full phone numbers, internal ledger notes and payout failure details are not returned to customers.

Admin > Refunds and returns > Customer refund requests shows the supplied information. Review can decline with a customer-visible explanation or approve up to the requested amount. Approval re-verifies the original IOTEC payer and payment, checks delivery history and remaining balance, then atomically creates exactly one reserved refund draft linked to the request. The admin reviews and confirms that draft using the existing payout flow; no details need to be retyped. A changed delivery stage requires reviewing the appropriate cancellation or return settlement type. Approval is not proof of payment: only provider-confirmed success becomes Refund paid. Rejected drafts and failed payouts remain visible for follow-up.
''')
