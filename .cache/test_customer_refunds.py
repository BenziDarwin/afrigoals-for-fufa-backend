from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def edit(p,a,b):
 s=p.read_text(encoding='utf-8');assert a in s,(str(p),a[:90]);p.write_text(s.replace(a,b),encoding='utf-8')
p=api/'routes/refund_database_test.go'
edit(p,'\trole := "admin"','\trole := "admin"\n actorID := userID')
edit(p,'c.Locals("userID", userID)','c.Locals("userID", actorID)')
edit(p,'\tRegisterRefunds(app, auth, Deps{DB: tx, Config: cfg, IoTec: services.NewIoTecClient(cfg)})','\tRegisterRefunds(app, auth, Deps{DB: tx, Config: cfg, IoTec: services.NewIoTecClient(cfg)})\n RegisterRefundRequests(app, auth, Deps{DB:tx,Config:cfg,IoTec:services.NewIoTecClient(cfg)})')
edit(p,'\tmakeDraft := func(','''  t.Run("customer requests and admin approval",func(t *testing.T){
   if err:=tx.SavePoint("customer_requests").Error;err!=nil{t.Fatal(err)}
   defer func(){tx.RollbackTo("customer_requests");role="admin";actorID=userID}()
   path:="/orders/"+order.ID+"/refund-requests"
   role="user";actorID=uuid.NewString()
   if code,body:=request("GET",path,nil);code!=404||strings.Contains(string(body),"original-tx"){t.Fatal("cross-account refund data exposed",code,string(body))}
   if code,_:=request("POST",path,map[string]any{"transactionId":"original-tx"});code!=404{t.Fatal("cross-account refund request accepted",code)}
   actorID=userID
   if code,body:=request("GET",path,nil);code!=200||!strings.Contains(string(body),"original-tx"){t.Fatal("own payment eligibility unavailable",code,string(body))}
   payload:=map[string]any{"transactionId":"original-tx","customerName":"Customer","phone":"0771234567","amount":800,"reason":"Request cancellation","details":"Please refund the duplicate item","receiptReference":"CUSTOMER-RECEIPT"}
   code,body:=request("POST",path,payload);if code!=201{t.Fatal(code,string(body))}
   if strings.Contains(string(body),"256771234567"){t.Fatal("full phone exposed to customer")}
   var customerRequest models.RefundRequest;json.Unmarshal(body,&customerRequest)
   if customerRequest.Status!="requested"{t.Fatal(customerRequest.Status)}
   if code,_:=request("POST",path,payload);code!=409{t.Fatal("duplicate open request accepted",code)}
   var count int64;tx.Model(&models.PaymentRefund{}).Count(&count);if count!=0||posts.Load()!=0{t.Fatal("request created a payout or reserved draft")}
   approve:=map[string]any{"amount":600,"kind":"cancellation","reviewNote":"Agreed partial refund with the customer"}
   if code,_:=request("POST","/admin/refund-requests/"+customerRequest.ID+"/approve",approve);code!=403{t.Fatal("customer approved their own refund",code)}
   role="admin"
   if code,body:=request("GET","/admin/refund-requests?status=requested",nil);code!=200||!strings.Contains(string(body),"256771234567")||!strings.Contains(string(body),"CUSTOMER-RECEIPT"){t.Fatal("admin missing customer details",code,string(body))}
   approve["amount"]=1000
   if code,_:=request("POST","/admin/refund-requests/"+customerRequest.ID+"/approve",approve);code!=409{t.Fatal("approved beyond customer request",code)}
   approve["amount"]=600
   code,body=request("POST","/admin/refund-requests/"+customerRequest.ID+"/approve",approve);if code!=200{t.Fatal(code,string(body))}
   var draft models.PaymentRefund;json.Unmarshal(body,&draft)
   if draft.RefundRequestID==nil||*draft.RefundRequestID!=customerRequest.ID||draft.Amount!=600||draft.Status!="draft"{t.Fatal("approval not linked to prepared refund",string(body))}
   if posts.Load()!=0{t.Fatal("approval sent money before final confirmation")}
   if code,_:=request("POST","/admin/refund-requests/"+customerRequest.ID+"/approve",approve);code!=409{t.Fatal("request approved twice",code)}
   if code,_:=request("POST","/admin/refund-requests/"+customerRequest.ID+"/decline",map[string]string{"reviewNote":"Declined after approval"});code!=409{t.Fatal("approved request could be declined",code)}
   role="user"
   code,body=request("GET",path,nil)
   if code!=200||!strings.Contains(string(body),`"refundStatus":"draft"`)||!strings.Contains(string(body),`"approvedAmount":600`)||strings.Contains(string(body),"256771234567"){t.Fatal("customer cannot track safe approval",code,string(body))}
   if code,_:=request("POST",path,payload);code!=409{t.Fatal("new request bypassed active approved refund",code)}
   // Withdraw the unsubmitted admin draft, then verify a bad payer can only be declined.
   role="admin";request("POST","/admin/refunds/"+draft.ID+"/reject",nil)
   role="user";payload["phone"]="0781234567"
   code,body=request("POST",path,payload);if code!=201{t.Fatal(code,string(body))}
   var wrongPayer models.RefundRequest;json.Unmarshal(body,&wrongPayer)
   role="admin"
   if code,_:=request("POST","/admin/refund-requests/"+wrongPayer.ID+"/approve",approve);code!=409{t.Fatal("wrong original payer approved",code)}
   if code,_:=request("POST","/admin/refund-requests/"+wrongPayer.ID+"/decline",map[string]string{"reviewNote":"Please provide the original payer number"});code!=200{t.Fatal(code)}
   role="user";code,body=request("GET",path,nil)
   if code!=200||!strings.Contains(string(body),"Please provide the original payer number")||!strings.Contains(string(body),`"status":"declined"`){t.Fatal("customer missing decline explanation",string(body))}
   if posts.Load()!=0{t.Fatal("review path sent money")}
  })
\tmakeDraft := func(''')
p=web/'tests/auth-protected.test.ts'
with p.open('a',encoding='utf-8') as f:f.write('''
test("guest refund requests cannot reach the backend",async()=>{
 setRequestCookies({});
 const route=await import("../app/api/orders/[id]/refund-requests/route");
 const calls=mockFetch(()=>json({}));
 const request=new Request("http://localhost/api/orders/o1/refund-requests",{method:"POST",body:JSON.stringify({amount:800})});
 const response=await route.POST(request,{params:Promise.resolve({id:"o1"})});
 assert.equal(response.status,401);assert.equal(calls.length,0);
});
''')
# This test file does not currently import the json helper.
edit(p,'headerValue, mockFetch, setRequestCookies','headerValue, json, mockFetch, setRequestCookies')
p=web/'tests/order-payment-status.test.ts'
with p.open('a',encoding='utf-8') as f:f.write('''
test("customer refund status distinguishes review, approval and verified payment",async()=>{
 const {refundRequestLabel}=await import("../lib/orders/refund-requests");
 assert.equal(refundRequestLabel({status:"requested",refundStatus:""}),"Awaiting admin review");
 assert.equal(refundRequestLabel({status:"declined",refundStatus:""}),"Request declined");
 assert.equal(refundRequestLabel({status:"approved",refundStatus:"draft"}),"Approved · awaiting refund submission");
 assert.equal(refundRequestLabel({status:"approved",refundStatus:"processing"}),"Approved · awaiting refund confirmation");
 assert.equal(refundRequestLabel({status:"approved",refundStatus:"succeeded"}),"Refund paid");
 assert.equal(refundRequestLabel({status:"approved",refundStatus:"failed"}),"Approved · payment needs attention");
});
''')
