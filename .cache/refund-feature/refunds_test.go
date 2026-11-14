package services

import (
 "context"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "testing"
)

func refundResponse() IoTecRefundResponse {
 return IoTecRefundResponse{IoTecCollectionResponse:IoTecCollectionResponse{TrackingID:"refund-tx",ExternalID:"case-1",WalletID:"wallet-1",Currency:"UGX",Amount:800,AmountPresent:true,Status:IoTecStatusSucceeded},Payee:"256771234567"}
}
func TestRefundVerification(t *testing.T) {
 for _, change:=range []struct{name string;change func(*IoTecRefundResponse)}{
  {"wrong amount",func(r *IoTecRefundResponse){r.Amount=900}},
  {"missing amount",func(r *IoTecRefundResponse){r.AmountPresent=false}},
  {"wrong recipient",func(r *IoTecRefundResponse){r.Payee="256771234568"}},
  {"same last digits different recipient",func(r *IoTecRefundResponse){r.Payee="256781234567"}},
  {"wrong wallet",func(r *IoTecRefundResponse){r.WalletID="other"}},
  {"missing wallet",func(r *IoTecRefundResponse){r.WalletID=""}},
  {"wrong currency",func(r *IoTecRefundResponse){r.Currency="USD"}},
  {"wrong case",func(r *IoTecRefundResponse){r.ExternalID="other"}},
  {"missing transaction",func(r *IoTecRefundResponse){r.TrackingID=""}},
 } {t.Run(change.name,func(t *testing.T){r:=refundResponse();change.change(&r);status,_:=VerifyRefundResponse(r,"case-1","wallet-1","256771234567",800);if status!="unknown"{t.Fatal(status)}})}
 r:=refundResponse()
 if s,_:=VerifyRefundResponse(r,"case-1","wallet-1","256771234567",800);s!="succeeded"{t.Fatal(s)}
 r.Status=IoTecStatusProcessing
 if s,_:=VerifyRefundResponse(r,"case-1","wallet-1","256771234567",800);s!="processing"{t.Fatal(s)}
}

func TestIoTecRefundContract(t *testing.T) {
 posts:=0
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  w.Header().Set("Content-Type","application/json")
  switch r.URL.Path {
  case "/token":w.Write([]byte(`{"access_token":"test","expires_in":300}`))
  case "/api/disbursements/disburse":
   posts++;var body map[string]any;json.NewDecoder(r.Body).Decode(&body)
   if body["payee"]!="256771234567" || body["externalId"]!="case-1" || body["walletId"]!="wallet-1" || body["currency"]!="UGX"{t.Errorf("invalid disbursement contract")}
   w.Write([]byte(`{"id":"refund-tx","externalId":"case-1","walletId":"wallet-1","currency":"UGX","amount":800,"payee":"256771234567","status":"Pending"}`))
  case "/api/disbursements/external-id/case-1":w.Write([]byte(`{"id":"refund-tx","externalId":"case-1","walletId":"wallet-1","currency":"UGX","amount":800,"payee":"256771234567","status":"Success"}`))
  case "/api/collections/status/original-tx":w.Write([]byte(`{"id":"original-tx","externalId":"original-1","walletId":"wallet-1","currency":"UGX","amount":800,"payer":"256771234567","status":"Success"}`))
  default:t.Errorf("unexpected path %s",r.URL.Path);w.WriteHeader(404)
  }
 }));defer server.Close()
 client:=NewIoTecClient(Config{IoTecBaseURL:server.URL,IoTecTokenURL:server.URL+"/token",IoTecClientID:"id",IoTecClientSecret:"secret",IoTecWalletID:"wallet-1"})
 phone,err:=client.VerifiedCollectionPayer(context.Background(),"original-tx",VerificationSnapshot{Amount:800,Currency:"UGX",ExpectedWalletID:"wallet-1",MerchantReference:"original-1"})
 if err!=nil || phone!="256771234567"{t.Fatal(phone,err)}
 if _,err=client.DisburseRefund(context.Background(),"case-1",phone,"Test Customer",800);err!=nil{t.Fatal(err)}
 response,err:=client.RefundStatus(context.Background(),"case-1");if err!=nil{t.Fatal(err)}
 status,_:=VerifyRefundResponse(response,"case-1","wallet-1",phone,800)
 if status!="succeeded" || posts!=1{t.Fatal(status,posts)}
}
