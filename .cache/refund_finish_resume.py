from pathlib import Path
import re
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def edit(p,a,b):
 s=p.read_text(encoding='utf-8');assert a in s,(str(p),a[:90]);p.write_text(s.replace(a,b),encoding='utf-8')
p=web/'app/(admin)/admin/refunds/page.tsx'
s=p.read_text();s,n=re.subn(r'reserved stock\.\s+After-delivery','reserved stock. Dispatched goods need return inspection before restocking. After-delivery',s);assert n==1;p.write_text(s,encoding='utf-8')
edit(api/'docs/refunds.md','Submission cancels the order and restores reserved stock once.','Submission cancels the order and restores unshipped reserved stock once. Dispatched goods need return inspection before restocking; a refund does not automatically make shipped items sellable.')
p=api/'routes/refund_database_test.go'
edit(p,'cfg.IoTecMobileMoneyEnabled = true','cfg.IoTecMobileMoneyEnabled = true\n cfg.IoTecCallbackSecret = "callback-test"')
edit(p,'\trole = "user"','\tif code,_:=request("POST","/payments/iotec/refund-callback",map[string]string{"externalId":"unknown"});code!=401{t.Fatal("unauthenticated refund callback accepted",code)}\n\trole = "user"')
edit(p,'if available, err := refundableAmount','''
 if code,_:=request("POST","/admin/refunds",makeDraft(500,"0771234567","return_settlement"));code!=409{t.Fatal("submitting refund allowed an additional payout",code)}
 if available, err := refundableAmount''')
