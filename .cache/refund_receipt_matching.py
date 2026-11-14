from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def edit(p,a,b):
 s=p.read_text(encoding='utf-8');assert a in s,(str(p),a[:90]);p.write_text(s.replace(a,b),encoding='utf-8')
(api/'routes/refund_receipts.go').write_text('''package routes

import (
 "context"
 "fmt"
 "strings"
 "afrigoals-api.com/models"
 "afrigoals-api.com/services"
)

// An SMS ID is not an IOTEC tracking UUID. Match the operator's ID only
// against independently verified collections, with exact original payer checks.
func resolveAdminRefundReceipt(deps Deps,receipt,phone string)(models.PaymentAttempt,error){
 receipt=strings.TrimSpace(receipt)
 if receipt==""||len(receipt)>150{return models.PaymentAttempt{},fmt.Errorf("transaction ID from the customer's payment SMS is required")}
 if deps.IoTec==nil{return models.PaymentAttempt{},fmt.Errorf("IOTEC is not configured")}
 var candidates []models.PaymentAttempt
 base:=deps.DB.Where("provider = ? AND status = ? AND verified_at IS NOT NULL AND masked_phone = ?","iotec","succeeded",services.MaskPhone(phone))
 if err:=base.Where("vendor_transaction_id = ?",receipt).Limit(26).Find(&candidates).Error;err!=nil{return models.PaymentAttempt{},err}
 if len(candidates)==0{
  // Old collections did not retain vendor IDs. Hydrate a bounded set from
  // IOTEC; a masked number alone never authorizes a match or refund.
  if err:=deps.DB.Where("provider = ? AND status = ? AND verified_at IS NOT NULL AND masked_phone = ? AND (vendor_transaction_id = ? OR vendor_transaction_id IS NULL)","iotec","succeeded",services.MaskPhone(phone),"").Order("created_at desc").Limit(26).Find(&candidates).Error;err!=nil{return models.PaymentAttempt{},err}
 }
 if len(candidates)>25{return models.PaymentAttempt{},fmt.Errorf("reconcile the original payment in Payments first so its SMS transaction ID can be linked")}
 ctx,cancel:=context.WithTimeout(context.Background(),providerCallTimeout);defer cancel()
 var found models.PaymentAttempt
 for _,attempt:=range candidates{
  payer,err:=deps.IoTec.VerifiedCollectionReceipt(ctx,attempt.ProviderReference,services.VerificationSnapshot{Amount:attempt.Amount,Currency:attempt.Currency,MerchantReference:attempt.MerchantReference,ExpectedWalletID:deps.Config.IoTecWalletID},receipt)
  if err!=nil||payer!=phone{continue}
  if found.ID!=""{return models.PaymentAttempt{},fmt.Errorf("SMS transaction ID is ambiguous; reconcile the original payment before refunding")}
  found=attempt
 }
 if found.ID==""{return found,fmt.Errorf("no verified successful IOTEC payment matches that SMS transaction ID and original payer")}
 if err:=deps.DB.Model(&found).Update("vendor_transaction_id",receipt).Error;err!=nil{return models.PaymentAttempt{},err}
 found.VendorTransactionID=receipt
 return found,nil
}
''',encoding='utf-8')
p=api/'routes/refunds.go';s=p.read_text()
a=s.index('\tvar attempt models.PaymentAttempt',s.index('func prepareRefund'));b=s.index('\trow := models.PaymentRefund{}',a)
s=s[:a]+'''    if deps.IoTec==nil{return refundError(c,503,"IOTEC is not configured")}
    attempt,err:=resolveAdminRefundReceipt(deps,req.TransactionID,phone)
    if err!=nil{return refundError(c,409,err.Error())}
'''+s[b:]
s=s.replace('PaymentAttemptID: attempt.ID, OriginalTransactionID: attempt.ProviderReference,','PaymentAttemptID: attempt.ID, OriginalTransactionID: attempt.ProviderReference, ReceiptReference:req.TransactionID,')
s=s.replace('deps.IoTec.VerifiedCollectionPayer(verifyCtx, original.ProviderReference, services.VerificationSnapshot{Amount: original.Amount, Currency: original.Currency, MerchantReference: original.MerchantReference, ExpectedWalletID: deps.Config.IoTecWalletID})','deps.IoTec.VerifiedCollectionReceipt(verifyCtx, original.ProviderReference, services.VerificationSnapshot{Amount: original.Amount, Currency: original.Currency, MerchantReference: original.MerchantReference, ExpectedWalletID: deps.Config.IoTecWalletID},draft.ReceiptReference)')
p.write_text(s,encoding='utf-8')
p=api/'routes/refund_requests.go'
edit(p,'len(body.ReceiptReference) > 150','len(body.ReceiptReference) == 0 || len(body.ReceiptReference) > 150')
edit(p,'"Provide your name, original payer number, reason and a whole UGX amount of at least 500"','"Provide the transaction ID from your payment SMS, your name, original payer number, reason and a whole UGX amount of at least 500"')
edit(p,'\tvar row models.RefundRequest\n\terr = deps.DB.Transaction','''    var original models.PaymentAttempt
    if deps.DB.Where("order_id = ? AND user_id = ? AND provider_reference = ? AND provider = ? AND status = ? AND verified_at IS NOT NULL",order.ID,order.UserID,body.TransactionID,"iotec","succeeded").First(&original).Error!=nil{return refundError(c,409,"Choose a verified payment belonging to this order")}
    if deps.IoTec==nil{return refundError(c,503,"IOTEC verification is temporarily unavailable")}
    ctx,cancel:=context.WithTimeout(context.Background(),providerCallTimeout);defer cancel()
    payer,verifyErr:=deps.IoTec.VerifiedCollectionReceipt(ctx,original.ProviderReference,services.VerificationSnapshot{Amount:original.Amount,Currency:original.Currency,MerchantReference:original.MerchantReference,ExpectedWalletID:deps.Config.IoTecWalletID},body.ReceiptReference)
    if verifyErr!=nil||payer!=phone{return refundError(c,409,"The SMS transaction ID or payer number does not match this order's verified payment. Check the original payment message.")}
\tvar row models.RefundRequest
\terr = deps.DB.Transaction''')
edit(p,'\t\tif order.PaymentMethod != mobileMoneyMethod {','\t\tactorID,_:=c.Locals("userID").(string)\n\t\tif order.UserID!=actorID{return fmt.Errorf("order not found")}\n\t\tif order.PaymentMethod != mobileMoneyMethod {')
edit(p,'\t\trow = models.RefundRequest{','\t\tif err:=tx.Model(&attempt).Update("vendor_transaction_id",body.ReceiptReference).Error;err!=nil{return err}\n\t\trow = models.RefundRequest{')
edit(p,'deps.IoTec.VerifiedCollectionPayer(ctx, attempt.ProviderReference, services.VerificationSnapshot{Amount: attempt.Amount, Currency: attempt.Currency, MerchantReference: attempt.MerchantReference, ExpectedWalletID: deps.Config.IoTecWalletID})','deps.IoTec.VerifiedCollectionReceipt(ctx, attempt.ProviderReference, services.VerificationSnapshot{Amount: attempt.Amount, Currency: attempt.Currency, MerchantReference: attempt.MerchantReference, ExpectedWalletID: deps.Config.IoTecWalletID},request.ReceiptReference)')
edit(p,'RefundRequestID: &requestID,','RefundRequestID: &requestID, ReceiptReference:request.ReceiptReference,')
# Customer SMS ID is manually entered and mandatory; the internal payment selector stays separate.
p=web/'components/orders/CustomerRefundRequests.tsx'
edit(p,'Original IOTEC transaction','Payment for this order')
edit(p,'{payment.transactionId} · up to','{payment.maskedPhone || "Verified payment"} · up to')
edit(p,'Mobile money receipt reference (optional)','Transaction ID from your payment SMS (required)')
edit(p,'placeholder="Reference from your payment message"','required\n                placeholder="Enter the transaction ID received from MTN or Airtel"')
edit(p,'Your receipt reference:','Your SMS transaction ID:')
p=web/'components/admin/AdminRefundRequests.tsx'
edit(p,'Customer receipt:','Customer SMS transaction ID:')
edit(p,'Original IOTEC transaction:','IOTEC tracking reference:')
p=web/'app/(admin)/admin/refunds/page.tsx'
edit(p,'Original IOTEC transaction ID','Transaction ID from customer’s payment SMS')
edit(p,'  providerStatusCode?: string;','  providerStatusCode?: string;\n  receiptReference?: string;')
edit(p,'Original transaction: {review.originalTransactionId}','SMS transaction ID: {review.receiptReference || "Not supplied — prepare a new draft with the customer’s SMS ID"}\n              <p>IOTEC tracking reference: {review.originalTransactionId}</p>')
# The register retains both identifiers; never replace tracking UUIDs with SMS IDs.
edit(p,'Payment: {row.originalTransactionId}','SMS ID: {row.receiptReference || "Legacy record"}\n                    <p>IOTEC payment: {row.originalTransactionId}</p>')
with (api/'docs/refunds.md').open('a',encoding='utf-8') as f:f.write('''

## Mandatory SMS transaction ID

Customers must enter the exact transaction ID from their original MTN/Airtel payment message. It is stored as receiptReference and compared against IOTEC's authoritative vendorTransactionId. It is distinct from IOTEC's tracking UUID (originalTransactionId/providerReference) and the merchant external ID. IDs are strings; numeric provider IDs are decoded without floating-point conversion. Never bind an entered TID to an order solely because a user supplied it.

Request submission, admin approval and final refund submission all independently verify the successful collection, SMS ID, exact payer, amount, wallet, currency and external reference. An IOTEC tracking UUID cannot substitute for the SMS transaction ID. New collection verification caches vendorTransactionId for lookup; old paid records can be rechecked in Payments to hydrate it. Admin manual refunds accept the SMS ID and original payer number, resolving old records through bounded provider checks if necessary. Legacy unsubmitted drafts with no SMS ID must be replaced with a verified draft. Existing completed/failed refunds remain available for status-only rechecks.
''')
