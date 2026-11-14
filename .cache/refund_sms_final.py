from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
p=web/'app/(admin)/admin/refunds/page.tsx';s=p.read_text(encoding='utf-8');s=s.replace('disabled={busy || !confirmed}','disabled={busy || !confirmed || !review.receiptReference}');s=s.replace('Â·','·').replace('â€”','—').replace('â€¦','…').replace('â€™','’');p.write_text(s,encoding='utf-8')
# Keep strict JSON parsing while preserving IDs represented as JSON numbers.
p=api/'services/iotec.go';s=p.read_text(encoding='utf-8');needle='\n\tout := IoTecCollectionResponse{';assert needle in s;s=s.replace(needle,'\n var trailing any\n if decoder.Decode(&trailing) != io.EOF { return IoTecCollectionResponse{}, errors.New("response contains trailing JSON data") }\n'+needle,1);p.write_text(s,encoding='utf-8')
