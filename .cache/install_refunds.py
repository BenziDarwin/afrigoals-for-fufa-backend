from pathlib import Path
import shutil
src=Path('F:/Afrigoals/Afrigoals-API/.cache/refund-feature')
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api')
web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def edit(p,old,new):
 s=p.read_text(encoding='utf-8');assert old in s,(str(p),old[:90]);p.write_text(s.replace(old,new),encoding='utf-8')
for source,target in {
 'payment_refund.go':api/'models/payment_refund.go',
 'iotec_refunds.go':api/'services/iotec_refunds.go',
 'refunds.go':api/'routes/refunds.go',
 'refund_reconciliation.go':api/'services/refund_reconciliation.go',
 'refunds_test.go':api/'services/iotec_refunds_test.go',
 'refund_database_test.go':api/'routes/refund_database_test.go',
 'page.tsx':web/'app/(admin)/admin/refunds/page.tsx',
 'refunds_route.ts':web/'app/api/admin/refunds/route.ts',
 'refund_action_route.ts':web/'app/api/admin/refunds/[id]/[action]/route.ts',
}.items():
 target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(src/source,target)
edit(api/'routes/routes.go','RegisterIoTecPayments(api, auth, deps)','RegisterIoTecPayments(api, auth, deps)\n\tRegisterRefunds(api, auth, deps)')
edit(api/'db/db.go','&models.PaymentAttempt{},','&models.PaymentAttempt{},\n\t\t&models.PaymentRefund{},')
edit(api/'routes/release_database_test.go','&models.PaymentAttempt{},','&models.PaymentAttempt{}, &models.PaymentRefund{},')
edit(api/'services/reconciliation.go','type ReconciliationReport struct {','type ReconciliationReport struct {\n\tRefunds RefundCashflow `json:"refunds"`')
edit(api/'services/reconciliation.go','\tvendors := map[string]*VendorReconciliation{}','\trefunds, err := BuildRefundCashflow(db, filter)\n\tif err != nil { return ReconciliationReport{}, err }\n\treport.Refunds = refunds\n\n\tvendors := map[string]*VendorReconciliation{}')
edit(web/'app/(admin)/admin/layout.tsx','{ label: "Payments", href: "/admin/payments", icon: CreditCard },','{ label: "Payments", href: "/admin/payments", icon: CreditCard },\n  { label: "Refunds and returns", href: "/admin/refunds", icon: ReceiptText },')
edit(web/'app/(admin)/admin/orders/page.tsx','Track purchases and update logistics stages.','Track purchases and update logistics stages. Refund paid cancellations or returns from the refunds register.')
edit(web/'app/(admin)/admin/orders/page.tsx','<div className="flex gap-2">','<div className="flex gap-2">\n          <Link href="/admin/refunds" className="rounded-md border px-3 py-2 text-sm">Refunds and returns</Link>')
edit(web/'app/(admin)/admin/payments/page.tsx','import { useCallback','import Link from "next/link";\nimport { useCallback')
# The payments register provides the original provider transaction ID for the refund form.
p=web/'app/(admin)/admin/payments/page.tsx'
s=p.read_text();needle='<div className="space-y-6">'
assert needle in s
s=s.replace(needle,needle+'\n      <Link href="/admin/refunds" className="inline-block rounded-lg border px-4 py-2 text-sm">Refunds and returns</Link>',1);p.write_text(s,encoding='utf-8')
p=web/'app/(admin)/admin/reconciliation/page.tsx'
edit(p,'  ordersTruncated?: boolean;','  ordersTruncated?: boolean;\n  refunds?: { collections: number; refundsPaid: number; refundsReserved: number; netCollections: number };')
edit(p,'    ["Paid orders", String(totals?.paidOrders ?? 0)],','    ["Verified IOTEC collections", formatPrice(report?.refunds?.collections ?? 0, currency)],\n    ["Verified refunds paid", formatPrice(report?.refunds?.refundsPaid ?? 0, currency)],\n    ["Refunds reserved / pending", formatPrice(report?.refunds?.refundsReserved ?? 0, currency)],\n    ["IOTEC collections less refunds", formatPrice(report?.refunds?.netCollections ?? 0, currency)],\n    ["Paid orders", String(totals?.paidOrders ?? 0)],')
edit(p,'and Afrigoals&apos; commission. Cancelled orders are excluded.','and Afrigoals&apos; commission. Sales figures are gross and exclude cancelled orders. IOTEC cash flow includes their collected payments and subtracts verified refunds; it excludes provider fees and other wallet movements. Cash flow uses verification dates; pending refunds use draft dates. Partial settlements are recorded separately from gross product sales.')
edit(p,'            Reconciliation\n','            Reconciliation\n')
# Admin status writes must share the order lock used by refund claims and payment verification.
p=api/'routes/admin_orders.go'
edit(p,'"gorm.io/gorm"','"gorm.io/gorm"\n "gorm.io/gorm/clause"')
edit(p,'\t\torder.Status = newStatus','''
        if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Items").First(&order, "id = ?", order.ID).Error; err != nil {
            tx.Rollback(); return refundError(c, 500, "Could not lock order")
        }
        oldStatus = normalizeOrderStatusName(order.Status)
        if isCancelledOrderStatus(oldStatus) && newStatus != "cancelled" {
            tx.Rollback(); return refundError(c, 409, "Cancelled orders cannot be reopened; review any refunds first")
        }
        if newStatus == "cancelled" && !isCancelledOrderStatus(oldStatus) {
            delivered, err := refundOrderWasDelivered(tx, order)
            if err != nil { tx.Rollback(); return refundError(c, 500, "Could not check delivery history") }
            if delivered { tx.Rollback(); return refundError(c, 409, "Delivered orders require a return settlement, not cancellation") }
            if err := restoreOrderStock(tx, order.Items); err != nil { tx.Rollback(); return refundError(c, 409, err.Error()) }
        }
        if order.PaymentMethod == mobileMoneyMethod && newStatus == "paid" && order.PaymentStatus != "paid" {
            tx.Rollback(); return refundError(c, 409, "Reconcile the IOTEC transaction to verify payment")
        }
        if order.PaymentStatus == "paid" && (newStatus == "unpaid" || newStatus == "payment_failed" || newStatus == "pending_payment") {
            tx.Rollback(); return refundError(c, 409, "A verified payment cannot be made unpaid; use the refund register")
        }
        order.Status = newStatus''')
edit(p,'if err := tx.Save(&order).Error;','if err := tx.Omit(clause.Associations).Save(&order).Error;')
# The alternate order status endpoint uses the same cancellation rules.
p=api/'routes/orders.go'
edit(p,'\t\tif newStatus == "cancelled" {\n\t\t\tif !canCancelOrder','''
        if isAdmin && order.PaymentMethod == mobileMoneyMethod && newStatus == "paid" && order.PaymentStatus != "paid" {
            tx.Rollback(); return refundError(c, 409, "Reconcile the IOTEC transaction to verify payment")
        }
        if order.PaymentStatus == "paid" && (newStatus == "unpaid" || newStatus == "payment_failed" || newStatus == "pending_payment") {
            tx.Rollback(); return refundError(c, 409, "A verified payment cannot be made unpaid; use the refund register")
        }
        if newStatus == "cancelled" {
            delivered, err := refundOrderWasDelivered(tx, order)
            if err != nil { tx.Rollback(); return refundError(c, 500, "Could not check delivery history") }
            if delivered { tx.Rollback(); return refundError(c, 409, "Delivered orders require a return settlement") }
\t\t\tif !canCancelOrder''')
# The refund ledger is a separate audit trail; original payment statuses stay paid.
(api/'docs/refunds.md').write_text('''# IOTEC refunds and returns

Admin > Refunds and returns accepts the original provider transaction ID (from Payments), original payer number, customer name, whole UGX amount, reason and settlement notes. The API re-fetches the successful collection and matches its wallet, currency, exact amount, external ID, transaction ID and payer. It never sends to a different number based on matching masked digits.

Prepare creates a draft, reserves the refundable balance and sends no money. Review and confirm submits an IOTEC mobile money disbursement. A cancellation refund is allowed only before delivery, checking both present status and delivery history again at submission. Submission cancels the order and restores reserved stock once. If the payout fails the order stays cancelled and the refund stays in the ledger. Delivered-product complaints use a documented return settlement with full or partial refunds; these do not restock goods automatically.

The original successful collection is immutable. Refunds have separate IDs, original and payout transaction IDs, amounts, reasons, settlement notes, administrator IDs, timestamps and statuses. Draft, submitting, processing, unknown and succeeded amounts reserve balance. Unknown or incomplete responses must not release funds for another attempt. Recheck looks up the original refund external ID; it never resends money. A failed refund can only be replaced after the failure is confirmed and a fresh draft is reviewed. Reject releases an unsubmitted draft.

The API requires at least UGX 500 per payout (IOTEC minimum). Refund totals cannot exceed the verified amount collected. Provider fees are separate wallet movements. Reconciliation shows gross verified IOTEC collections (including cancelled orders), verified refunds, reserved refunds and collections less refunds. These cash-flow totals use verification dates, reservations use creation dates, and neither substitutes for the actual wallet balance. Product/vendor reports remain gross; partial settlement amounts are shown separately rather than guessed against individual products.

No refund is automatically issued merely by changing an order status. Use the refund register and review the customer agreement first. No client messages or live payouts are sent by the tests. The disbursement credential must have wallet payout permission; IOTEC may additionally require portal approval. Use Recheck until provider confirmation arrives. Current submission-time crash recovery deliberately remains reserved until IOTEC confirms it; do not manually reset a submitting/unknown record or blindly retry it.

Official provider contract: https://pay.iotec.io/api-docs/index.html (POST /api/disbursements/disburse; GET /api/disbursements/external-id/{externalId}).
''',encoding='utf-8')
