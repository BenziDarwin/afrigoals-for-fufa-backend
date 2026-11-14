from pathlib import Path
import re
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def edit(p,a,b):
 s=p.read_text(encoding='utf-8');assert a in s,(str(p),a[:85]);p.write_text(s.replace(a,b),encoding='utf-8')
p=web/'app/(admin)/admin/refunds/page.tsx'
s=p.read_text(encoding='utf-8');s=re.sub(r'<p>\s*IOTEC payment: \{row.originalTransactionId\}\s*</p>','<span className="block">IOTEC payment: {row.originalTransactionId}</span>',s);p.write_text(s,encoding='utf-8')
source=Path('F:/Afrigoals/Afrigoals-API/.cache/refund_sms_tests.py').read_text(encoding='utf-8')
start=source.index("p=api/'routes/refund_database_test.go'")
exec(source[start:].replace('.read_text()',".read_text(encoding='utf-8')"))
