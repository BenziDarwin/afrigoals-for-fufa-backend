from pathlib import Path
p=Path('F:/Afrigoals/Afrigoals_ecommerce_api/routes/refund_database_test.go');s=p.read_text()
a='\tvar posts atomic.Int32';assert a in s;s=s.replace(a,a+'\n var reversed atomic.Bool')
a='\t\tcase r.URL.Path == "/api/collections/status/original-tx":';assert a in s;s=s.replace(a,a+'\n if reversed.Load() { fmt.Fprint(w,`{"id":"original-tx","externalId":"original-1","walletId":"wallet-1","currency":"UGX","amount":2000,"payer":"256771234567","status":"Failed"}`);return }')
a='\tcode, body = request("POST", "/admin/refunds/"+first.ID+"/submit", map[string]bool{"confirm": true})';assert a in s;s=s.replace(a,'''
 reversed.Store(true)
 if code,_:=request("POST","/admin/refunds/"+first.ID+"/submit",map[string]bool{"confirm":true});code!=409 || posts.Load()!=0 {t.Fatal("reversed original payment was refunded",code,posts.Load())}
 reversed.Store(false)
'''+a,1)
p.write_text(s,encoding='utf-8')
