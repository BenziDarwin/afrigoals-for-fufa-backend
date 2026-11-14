from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def edit(p,a,b):
 s=p.read_text(encoding='utf-8');assert a in s,(str(p),a[:85]);p.write_text(s.replace(a,b),encoding='utf-8')
edit(api/'models/payment_attempt.go','\tProviderReference string','\tVendorTransactionID string `json:"vendorTransactionId" gorm:"index"`\n\tProviderReference string')
edit(api/'models/payment_refund.go','type PaymentRefund struct {','type PaymentRefund struct {\n ReceiptReference string `json:"receiptReference" gorm:"index"`')
p=api/'services/iotec.go'
edit(p,'\tTrackingID    string','\tVendorTransactionID string\n\tTrackingID    string')
edit(p,'if err := json.Unmarshal(raw, &fields); err != nil {\n\t\treturn IoTecCollectionResponse{}, errors.New("response is not a JSON object")','decoder := json.NewDecoder(bytes.NewReader(raw))\n decoder.UseNumber()\n if err := decoder.Decode(&fields); err != nil {\n\t\treturn IoTecCollectionResponse{}, errors.New("response is not a JSON object")')
edit(p,'\t\tTrackingID: firstField','\t\tVendorTransactionID: firstField(fields, "vendorTransactionId"),\n\t\tTrackingID: firstField')
edit(p,'\t\tcase float64:\n\t\t\treturn strconv.FormatFloat','\t\tcase json.Number:\n\t\t\treturn t.String()\n\t\tcase float64:\n\t\t\treturn strconv.FormatFloat')
# UseNumber also affects amount decoding.
edit(p,'func numberOK(v any) (float64, bool) {\n\tswitch t := v.(type) {','func numberOK(v any) (float64, bool) {\n\tswitch t := v.(type) {\n case json.Number:\n  n,err:=t.Float64();return n,err==nil')
p=api/'services/iotec_refunds.go'
# Preserve the existing API while adding a strict receipt-ID verification method.
edit(p,'func (c *IoTecClient) VerifiedCollectionPayer(ctx context.Context, reference string, snap VerificationSnapshot) (string, error) {','''func (c *IoTecClient) VerifiedCollectionPayer(ctx context.Context, reference string, snap VerificationSnapshot) (string, error) {
 phone,_,err:=c.verifiedCollectionDetails(ctx,reference,snap)
 return phone,err
}

func (c *IoTecClient) VerifiedCollectionReceipt(ctx context.Context, reference string, snap VerificationSnapshot, receipt string) (string,error) {
 receipt=strings.TrimSpace(receipt)
 if receipt=="" || len(receipt)>150{return "",fmt.Errorf("the transaction ID from the payment SMS is required")}
 phone,vendorID,err:=c.verifiedCollectionDetails(ctx,reference,snap)
 if err!=nil{return "",err}
 if vendorID=="" || vendorID!=receipt{return "",fmt.Errorf("SMS transaction ID does not match the verified IOTEC collection")}
 return phone,nil
}

func (c *IoTecClient) verifiedCollectionDetails(ctx context.Context, reference string, snap VerificationSnapshot) (string,string,error) {''')
s=p.read_text();a=s.index('func (c *IoTecClient) verifiedCollectionDetails');b=s.index('\nfunc (c *IoTecClient) DisburseRefund',a);part=s[a:b]
part=part.replace('return "", err','return "", "", err').replace('return "", fmt.Errorf','return "", "", fmt.Errorf').replace('return phone, nil','return phone, response.VendorTransactionID, nil');s=s[:a]+part+s[b:];p.write_text(s,encoding='utf-8')
# Cache only independently verified vendor IDs, including explicit rechecks of old successful payments.
p=api/'routes/iotec_payments.go'
needle='\tif locked.Status == attemptStatusSucceeded {\n\t\ttx.Rollback()'
edit(p,needle,'''    if locked.Status == attemptStatusSucceeded && next == attemptStatusSucceeded && resp.VendorTransactionID != "" {
        if err:=tx.Model(&locked).Update("vendor_transaction_id",resp.VendorTransactionID).Error;err!=nil{tx.Rollback();return locked,err}
        locked.VendorTransactionID=resp.VendorTransactionID
        if err:=tx.Commit().Error;err!=nil{return locked,err}
        return locked,nil
    }
'''+needle)
edit(p,'\tattemptUpdates := map[string]any{','\tattemptUpdates := map[string]any{')
edit(p,'\tif locked.ProviderReference == "" {\n\t\tattemptUpdates','\tif next == attemptStatusSucceeded && resp.VendorTransactionID != "" {attemptUpdates["vendor_transaction_id"]=resp.VendorTransactionID}\n\tif locked.ProviderReference == "" {\n\t\tattemptUpdates')
