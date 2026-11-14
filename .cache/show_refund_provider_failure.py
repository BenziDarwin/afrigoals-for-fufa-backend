from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def edit(p,a,b):
 s=p.read_text(encoding='utf-8');assert a in s,(str(p),a[:90]);p.write_text(s.replace(a,b),encoding='utf-8')
p=api/'models/payment_refund.go'
edit(p,'\tProviderStatus        string', '\tProviderStatusCode string `json:"providerStatusCode"`\n\tProviderStatus        string') if '\tProviderStatus        string' in p.read_text() else edit(p,'\tProviderStatus ', '\tProviderStatusCode string `json:"providerStatusCode"`\n\tProviderStatus ')
p=api/'services/iotec_refunds.go'
edit(p,'\tPayee string','\tProviderStatusCode string\n\tPayee string')
edit(p,'\t\tPayee string `json:"payee"`','\t\tPayee string `json:"payee"`\n\t\tStatusCode string `json:"statusCode"`')
edit(p,'IoTecRefundResponse{IoTecCollectionResponse: response, Payee: fields.Payee}','IoTecRefundResponse{IoTecCollectionResponse: response, Payee: fields.Payee, ProviderStatusCode: safeRefundProviderCode(fields.StatusCode)}')
edit(p,'return "failed", "IOTEC reported that the refund did not complete"','return "failed", refundFailureReason(resp.ProviderStatusCode)')
with p.open('a',encoding='utf-8') as f:f.write('''
// Preserve machine codes without passing raw provider messages or payloads to
// the UI. Human messages may contain account or callback details.
func safeRefundProviderCode(value string) string {
 value=strings.TrimSpace(value)
 if len(value)>80{return ""}
 for _,ch:=range value{if !((ch>='a'&&ch<='z')||(ch>='A'&&ch<='Z')||(ch>='0'&&ch<='9')||ch=='-'||ch=='_'||ch=='.'){return ""}}
 return value
}
func refundFailureReason(code string) string {
 code=safeRefundProviderCode(code)
 switch strings.ToLower(code) {
 case "airtel-error":return "Refund failed: Airtel reported an external vendor error. Recheck the existing transaction or contact IOTEC support with its transaction ID."
 }
 if code!=""{return "IOTEC reported that the refund did not complete (provider code: "+code+")"}
 return "IOTEC reported that the refund did not complete"
}
''')
edit(api/'routes/refunds.go','\t\trow.ProviderStatus = response.RawStatus','\t\trow.ProviderStatus = response.RawStatus\n\t\trow.ProviderStatusCode = response.ProviderStatusCode')
p=web/'app/(admin)/admin/refunds/page.tsx'
edit(p,'  providerReference: string;','  providerReference: string;\n  providerStatusCode?: string;')
# Render alongside the current failure reason so all admin rechecks expose the code.
edit(p,'{row.failureReason}','{row.failureReason}\n                    {row.providerStatusCode && <span className="block">Provider code: {row.providerStatusCode}</span>}')
p=api/'services/iotec_refunds_test.go'
with p.open('a',encoding='utf-8') as f:f.write('''
func TestRefundFailureRetainsProviderCodeWithoutRawMessages(t *testing.T) {
 client:=NewIoTecClient(Config{})
 response,err:=client.interpretRefund([]byte(`{"id":"refund-tx","externalId":"case-1","walletId":"wallet-1","currency":"UGX","amount":800,"payee":"256771234567","status":"Failed","statusCode":"Airtel-error","statusMessage":"External Vendor error - private credential must not reach UI"}`),200)
 if err!=nil{t.Fatal(err)}
 status,reason:=VerifyRefundResponse(response,"case-1","wallet-1","256771234567",800)
 if status!="failed"||response.ProviderStatusCode!="Airtel-error"||!strings.Contains(reason,"Airtel"){t.Fatal(status,reason)}
 if strings.Contains(reason,"private credential"){t.Fatal("raw provider payload leaked")}
 response.Amount=900
 if status,_:=VerifyRefundResponse(response,"case-1","wallet-1","256771234567",800);status!="unknown"{t.Fatal("failure code bypassed transaction verification")}
 for _,code:=range []string{"invalid code with spaces","<script>",strings.Repeat("x",81)}{if safeRefundProviderCode(code)!=""{t.Fatal("unsafe code retained")}}
}
''')
edit(p,'"net/http/httptest"','"net/http/httptest"\n"strings"')
with (api/'docs/refunds.md').open('a',encoding='utf-8') as f:f.write('''

Provider failure codes are retained on the admin ledger as providerStatusCode after an authenticated provider recheck. Airtel-error is shown as an external vendor failure; it does not identify the operator's underlying cause or justify automatically resending money. Use Recheck on the existing refund after deploying this update to refresh its displayed code. Original payouts are never retried by that action. Contact IOTEC with the existing provider transaction ID if the failure remains unresolved.
''')
