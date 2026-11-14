from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def edit(p,a,b):
 s=p.read_text(encoding='utf-8');assert a in s,(str(p),a[:85]);p.write_text(s.replace(a,b),encoding='utf-8')
edit(api/'routes/iotec_payments.go','if attempt.Status == attemptStatusSucceeded {\n\t\treturn attempt, nil','if attempt.Status == attemptStatusSucceeded && attempt.VendorTransactionID != "" {\n\t\treturn attempt, nil')
p=web/'app/(admin)/admin/refunds/page.tsx';s=p.read_text();s=s.replace('<p>IOTEC payment: {row.originalTransactionId}</p>','<span className="block">IOTEC payment: {row.originalTransactionId}</span>');p.write_text(s,encoding='utf-8')
p=api/'routes/refund_database_test.go';s=p.read_text()
s=s.replace('ProviderReference: "original-tx", Amount: 2000','ProviderReference: "original-tx", MaskedPhone:services.MaskPhone("256771234567"), Amount: 2000')
s=s.replace('"payer":"256771234567","status":"Success"','"payer":"256771234567","vendorTransactionId":"160875391949","status":"Success"')
s=s.replace('CUSTOMER-RECEIPT','160875391949')
a=s.index('\t\t// Withdraw the unsubmitted admin draft');b=s.index('\n\t\tif posts.Load() != 0',a)
s=s[:a]+'''        // Incorrect payer and SMS IDs are rejected before entering the queue.
        role="admin";request("POST","/admin/refunds/"+draft.ID+"/reject",nil)
        role="user";payload["phone"]="0781234567"
        if code,_:=request("POST",path,payload);code!=409{t.Fatal("wrong payer request accepted",code)}
        payload["phone"]="0771234567";payload["receiptReference"]="WRONG-SMS-ID"
        if code,_:=request("POST",path,payload);code!=409{t.Fatal("wrong SMS request accepted",code)}
        payload["receiptReference"]=""
        if code,_:=request("POST",path,payload);code!=400{t.Fatal("missing SMS ID accepted",code)}
        payload["receiptReference"]="original-tx"
        if code,_:=request("POST",path,payload);code!=409{t.Fatal("tracking UUID accepted as SMS ID",code)}
        payload["receiptReference"]="160875391949"
        code,body=request("POST",path,payload);if code!=201{t.Fatal(code,string(body))}
        var secondRequest models.RefundRequest;json.Unmarshal(body,&secondRequest)
        role="admin"
        if code,_:=request("POST","/admin/refund-requests/"+secondRequest.ID+"/decline",map[string]string{"reviewNote":"Please contact us to discuss the requested settlement"});code!=200{t.Fatal(code)}
        role="user";code,body=request("GET",path,nil)
        if code!=200||!strings.Contains(string(body),"Please contact us to discuss")||!strings.Contains(string(body),`"status":"declined"`){t.Fatal("customer missing decline explanation",string(body))}
'''+s[b:]
# Only manual admin requests use the SMS ID as transactionId. Customer requests keep the internal payment selection separate.
a=s.index('\tmakeDraft := func(');s=s[:a]+s[a:].replace('"transactionId": "original-tx"','"transactionId": "160875391949"')
s=s.replace('makeDraft(600, "0781234567", "cancellation")); code != 400','makeDraft(600, "0781234567", "cancellation")); code != 409')
p.write_text(s,encoding='utf-8')
p=api/'services/iotec_refunds_test.go'
with p.open('a',encoding='utf-8') as f:f.write('''
func TestSMSReceiptMustMatchVendorTransactionID(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if r.URL.Path=="/token"{fmt.Fprint(w,`{"access_token":"mock","expires_in":300}`);return}
  fmt.Fprint(w,`{"id":"tracking-uuid","externalId":"merchant-1","walletId":"wallet-1","currency":"UGX","amount":800,"payer":"256771234567","vendorTransactionId":"160875391949","status":"Success"}`)
 }));defer server.Close()
 client:=NewIoTecClient(Config{IoTecBaseURL:server.URL,IoTecTokenURL:server.URL+"/token",IoTecClientID:"id",IoTecClientSecret:"secret",IoTecWalletID:"wallet-1"})
 snapshot:=VerificationSnapshot{Amount:800,Currency:"UGX",MerchantReference:"merchant-1",ExpectedWalletID:"wallet-1"}
 for _,bad:=range []string{"","tracking-uuid","merchant-1","160875391948"}{
  if _,err:=client.VerifiedCollectionReceipt(context.Background(),"tracking-uuid",snapshot,bad);err==nil{t.Fatal("unverified receipt accepted",bad)}
 }
 phone,err:=client.VerifiedCollectionReceipt(context.Background(),"tracking-uuid",snapshot," 160875391949 ")
 if err!=nil||phone!="256771234567"{t.Fatal(phone,err)}
}

func TestVendorTransactionIDPreservesNumericPrecision(t *testing.T){
 response,err:=ParseIoTecCollectionResponse([]byte(`{"id":"tracking","vendorTransactionId":123456789012345678901234,"amount":800,"status":"Success"}`))
 if err!=nil||response.VendorTransactionID!="123456789012345678901234"||response.Amount!=800{t.Fatal(response,err)}
}
''')
