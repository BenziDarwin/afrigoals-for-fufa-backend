from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def edit(p,a,b):
 s=p.read_text(encoding='utf-8');assert a in s,(str(p),a[:90]);p.write_text(s.replace(a,b),encoding='utf-8')
p=api/'services/iotec_refunds.go'
edit(p,'\t\tProviderStatusCode string\n\tPayee string `json:"payee"`','\t\tPayee string `json:"payee"`')
source=Path('F:/Afrigoals/Afrigoals-API/.cache/show_refund_provider_failure.py').read_text()
start=source.index("edit(p,'\\t\\tPayee string")
exec(source[start:])
