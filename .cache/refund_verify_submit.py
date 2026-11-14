from pathlib import Path
p=Path('F:/Afrigoals/Afrigoals_ecommerce_api/routes/refunds.go')
s=p.read_text();old='\trow, err := claimRefund(deps.DB, c.Params("id"), c.Locals("userID").(string))'
new='''
 // A draft may be reviewed much later; reverify the original payment before
 // committing a payout, in case the collection has since been reversed.
 var draft models.PaymentRefund
 if deps.DB.First(&draft, "id = ?", c.Params("id")).Error != nil { return refundError(c,404,"Refund not found") }
 if draft.Status != "draft" { return refundError(c,409,"Refund already submitted or rejected; use Recheck instead") }
 var original models.PaymentAttempt
 if deps.DB.First(&original, "id = ? AND status = ? AND verified_at IS NOT NULL",draft.PaymentAttemptID,"succeeded").Error != nil { return refundError(c,409,"Original payment is not verified for refund") }
 verifyCtx,verifyCancel:=context.WithTimeout(context.Background(),providerCallTimeout)
 payer,verifyErr:=deps.IoTec.VerifiedCollectionPayer(verifyCtx,original.ProviderReference,services.VerificationSnapshot{Amount:original.Amount,Currency:original.Currency,MerchantReference:original.MerchantReference,ExpectedWalletID:deps.Config.IoTecWalletID})
 verifyCancel()
 if verifyErr != nil || payer != draft.Phone || original.OrderID != draft.OrderID || original.ProviderReference != draft.OriginalTransactionID { return refundError(c,409,"Original payment or payer could not be reverified. No refund was sent.") }
\trow, err := claimRefund(deps.DB, c.Params("id"), c.Locals("userID").(string))'''
assert old in s;p.write_text(s.replace(old,new),encoding='utf-8')
# Avoid an incorrect stock-restored message when goods are already dispatched.
p=Path('F:/Afrigoals/Afrigoals_ecommerce_api/routes/orders.go');s=p.read_text();s=s.replace('note = "Order cancelled and stock restored"','note = "Order cancelled; unshipped stock released. Inspect dispatched goods before restocking."');p.write_text(s,encoding='utf-8')
