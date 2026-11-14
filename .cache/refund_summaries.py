from pathlib import Path
import shutil
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def edit(p,a,b):
 s=p.read_text(encoding='utf-8');assert a in s,(str(p),a[:80]);p.write_text(s.replace(a,b),encoding='utf-8')
shutil.copyfile('F:/Afrigoals/Afrigoals-API/.cache/refund-feature/refund_summaries.go',api/'services/refund_summaries.go')
edit(api/'models/order.go','type Order struct {','type Order struct {\n RefundedAmount int64 `json:"refundedAmount" gorm:"-"`\n RefundPendingAmount int64 `json:"refundPendingAmount" gorm:"-"`')
for name in ['routes/admin_orders.go','routes/orders.go']:
 p=api/name
 edit(p,'\t\treturn c.JSON(orders)','\t\tif err := services.AttachRefundSummaries(deps.DB, orders); err != nil { return refundError(c, 500, "Could not load refund status") }\n\t\treturn c.JSON(orders)')
p=api/'routes/orders.go'
# Restrict augmentation to the authenticated detail route, after ownership query.
s=p.read_text();start=s.index('router.Get("/orders/:id"');before=s[:start];tail=s[start:]
needle='\t\treturn c.JSON(order)';assert needle in tail
tail=tail.replace(needle,'\t\tsummary := []models.Order{order}\n\t\tif err := services.AttachRefundSummaries(deps.DB, summary); err != nil { return refundError(c, 500, "Could not load refund status") }\n\t\treturn c.JSON(summary[0])',1);p.write_text(before+tail,encoding='utf-8')
edit(web/'lib/api/types.ts','  paymentStatus?: string;','  paymentStatus?: string;\n  refundedAmount?: number;\n  refundPendingAmount?: number;')
p=web/'lib/orders/status.ts'
with p.open('a',encoding='utf-8') as f:f.write('''
export function formatOrderPayment(order: {status: string;paymentStatus?:string;paidAt?:string|null;total:number;refundedAmount?:number;refundPendingAmount?:number}) {
 if ((order.refundedAmount ?? 0) > 0) return (order.refundedAmount ?? 0) >= order.total ? "Refunded" : "Partially refunded";
 if ((order.refundPendingAmount ?? 0) > 0) return "Paid · refund pending";
 return orderIsPaid(order) ? "Paid" : formatOrderStatus(order.paymentStatus || "unpaid");
}
''')
p=web/'app/(admin)/admin/orders/page.tsx'
edit(p,'  orderIsPaid,','  orderIsPaid,\n  formatOrderPayment,')
edit(p,'{orderIsPaid(o)\n                        ? "Paid"\n                        : formatOrderStatus(o.paymentStatus || "unpaid")}','{formatOrderPayment(o)}')
p=web/'app/(app)/orders/[id]/page.tsx'
edit(p,'  paymentStatus?: string;','  paymentStatus?: string;\n  refundedAmount?: number;\n  refundPendingAmount?: number;')
edit(p,'            <div className="mt-4 grid gap-3 text-sm sm:grid-cols-2">','''            {(order.refundedAmount ?? 0) > 0 && <p className="mt-4 rounded-lg border p-3 text-sm">Refund paid: {formatPrice(order.refundedAmount ?? 0, "ugx")}. {Number(order.refundedAmount) >= Number(order.total) ? "Full refund confirmed." : "Partial refund confirmed."}</p>}
            {(order.refundPendingAmount ?? 0) > 0 && <p className="mt-4 rounded-lg border p-3 text-sm">Refund under review or awaiting payment confirmation: {formatPrice(order.refundPendingAmount ?? 0, "ugx")}.</p>}
            <div className="mt-4 grid gap-3 text-sm sm:grid-cols-2">''')
# Tests exercise the visible outcome rather than treating historical Paid as the refund result.
p=web/'tests/order-payment-status.test.ts'
edit(p,'effectiveOrderStage,','formatOrderPayment, effectiveOrderStage,')
with p.open('a',encoding='utf-8') as f:f.write('''
test("refund results remain separate from original verified payment", () => {
 const order={status:"cancelled",paymentStatus:"paid",total:2000};
 assert.equal(formatOrderPayment({...order,refundedAmount:2000}),"Refunded");
 assert.equal(formatOrderPayment({...order,refundedAmount:800}),"Partially refunded");
 assert.equal(formatOrderPayment({...order,refundPendingAmount:800}),"Paid · refund pending");
 assert.equal(formatOrderPayment(order),"Paid");
});
''')
# Remove unused import introduced in the isolated database test.
p=api/'routes/refund_database_test.go'
edit(p,'\t"context"\n','')
edit(p,'\t_ = context.Background()\n','')
