from pathlib import Path
web=Path('F:/Afrigoals/Afrigoals_ecommerce')
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api')
def edit(p,old,new):
 s=p.read_text(encoding='utf-8'); assert old in s, (p,old); p.write_text(s.replace(old,new),encoding='utf-8')
p=web/'lib/orders/status.ts'
edit(p,'  "pending",','  "pending",\n  "pending_payment",\n  "confirmed",')
with p.open('a',encoding='utf-8') as f: f.write('''
export function orderIsPaid(order: { paymentStatus?: string; status: string; paidAt?: string | null }) {
  return normalizeOrderStatus(order.paymentStatus) === "paid" ||
    (!order.paymentStatus && (Boolean(order.paidAt) || normalizeOrderStatus(order.status) === "paid"));
}

export function effectiveOrderStage(order: { paymentStatus?: string; status: string; paidAt?: string | null }) {
  const stage = normalizeOrderStatus(order.status);
  if (orderIsPaid(order) && ["unpaid", "pending", "pending_payment", "confirmed", "payment_failed", "paid"].includes(stage)) return "paid";
  return stage === "canceled" ? "cancelled" : stage;
}
''')
edit(web/'lib/api/types.ts','  paymentMethod: string;','  paymentMethod: string;\n  paymentStatus?: string;')
p=web/'app/(admin)/admin/orders/page.tsx'
edit(p,'ORDER_STATUSES, formatOrderStatus','ORDER_STATUSES, formatOrderStatus, effectiveOrderStage, orderIsPaid')
edit(p,'value={o.status}','value={effectiveOrderStage(o)}')
edit(p,'<th className="px-4 py-3">Stage</th>','<th className="px-4 py-3">Payment</th>\n              <th className="px-4 py-3">Stage</th>')
edit(p,'colSpan={5}','colSpan={6}')
edit(p,'                    <select\n','                    <select\n')
edit(p,'                  <td className="px-4 py-3">\n                    <select','                  <td className="px-4 py-3">\n                    <span className={orderIsPaid(o) ? "font-medium text-green-700 dark:text-green-400" : "text-zinc-500"}>\n                      {orderIsPaid(o) ? "Paid" : formatOrderStatus(o.paymentStatus || "unpaid")}\n                    </span>\n                  </td>\n                  <td className="px-4 py-3">\n                    <select')
edit(p,'                      {ORDER_STATUSES.map((s) => (','                      {!ORDER_STATUSES.includes(effectiveOrderStage(o) as typeof ORDER_STATUSES[number]) && <option value={effectiveOrderStage(o)}>{formatOrderStatus(o.status)}</option>}\n                      {ORDER_STATUSES.map((s) => (')
p=api/'services/orders.go'
edit(p,'\t"pending":          {},','\t"pending":          {},\n\t"pending_payment": {},\n\t"confirmed": {},')
p=api/'routes/iotec_payments.go'
edit(p,'order.Status = "confirmed"','order.Status = successfulPaymentOrderStage(order.Status)')
edit(p,'"status":         "confirmed",','"status":         order.Status,')
edit(p,'Status:  "confirmed",','Status:  order.Status,')
edit(p,'\t\torder.Status = successfulPaymentOrderStage(order.Status)\n\t\tgo sendOrderStatusUpdateEmail(order, oldOrderStatus, "confirmed")','\t\tgo sendOrderStatusUpdateEmail(order, oldOrderStatus, order.Status)')
with p.open('a',encoding='utf-8') as f: f.write('''
// A payment confirmation must not undo an existing fulfilment stage.
func successfulPaymentOrderStage(status string) string {
 switch strings.ToLower(strings.TrimSpace(status)) {
 case "processing", "packed", "shipped", "out_for_delivery", "delivered", "cancelled", "canceled":
  return status
 default:
  return "paid"
 }
}
''')
(api/'routes/paid_stage_test.go').write_text('''package routes
import "testing"
func TestSuccessfulPaymentOrderStage(t *testing.T) {
 for _, s := range []string{"unpaid", "pending_payment", "confirmed", "payment_failed"} {
  if got := successfulPaymentOrderStage(s); got != "paid" { t.Fatalf("%s became %s", s, got) }
 }
 for _, s := range []string{"processing", "packed", "shipped", "out_for_delivery", "delivered", "cancelled"} {
  if got := successfulPaymentOrderStage(s); got != s { t.Fatalf("payment regressed %s to %s", s, got) }
 }
}
''',encoding='utf-8')
(web/'tests/order-payment-status.test.ts').write_text('''import { test } from "node:test";
import assert from "node:assert/strict";
import { effectiveOrderStage, orderIsPaid } from "../lib/orders/status";
test("verified online payments display Paid for legacy and new early stages", () => {
 for (const status of ["confirmed", "pending_payment", "unpaid", "paid"]) assert.equal(effectiveOrderStage({status, paymentStatus:"paid"}), "paid");
});
test("pending payments are not shown as paid", () => {
 assert.equal(orderIsPaid({status:"confirmed", paymentStatus:"pending"}), false);
 assert.equal(effectiveOrderStage({status:"pending_payment", paymentStatus:"pending"}), "pending_payment");
});
test("payment badge stays Paid while fulfilment advances", () => {
 for (const status of ["processing", "shipped", "delivered", "cancelled"]) {
  const order = {status, paymentStatus:"paid"};
  assert.equal(effectiveOrderStage(order), status);
  assert.equal(orderIsPaid(order), true);
 }
});
''',encoding='utf-8')
edit(web/'package.json','tests/commerce-growth.test.ts"','tests/commerce-growth.test.ts tests/order-payment-status.test.ts"')
