from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def edit(p,a,b):
 s=p.read_text(encoding='utf-8');assert a in s,(str(p),a[:90]);p.write_text(s.replace(a,b),encoding='utf-8')
for name in ['routes/refunds.go','routes/orders.go','routes/admin_orders.go']:
 edit(api/name,'restoreOrderStock(tx, order.Items)','restoreCancellationStock(tx, order)')
with (api/'routes/refunds.go').open('a',encoding='utf-8') as f:f.write('''
// Goods already dispatched must be inspected on return before restocking.
func restoreCancellationStock(tx *gorm.DB, order models.Order) error {
 if normalizeOrderStatusName(order.Status)=="shipped" || normalizeOrderStatusName(order.Status)=="out_for_delivery" { return nil }
 var count int64
 if err:=tx.Model(&models.OrderStatusEvent{}).Where("order_id = ? AND LOWER(status) IN ?",order.ID,[]string{"shipped","out_for_delivery","delivered"}).Count(&count).Error;err!=nil{return err}
 if count>0{return nil}
 if err:=tx.Model(&models.OrderDeliveryAssignment{}).Where("order_id = ? AND (picked_up_at IS NOT NULL OR delivered_at IS NOT NULL OR status IN ?)",order.ID,[]string{"picked_up","delivered"}).Count(&count).Error;err!=nil{return err}
 if count>0{return nil}
 return restoreOrderStock(tx,order.Items)
}
''')
edit(web/'app/(admin)/admin/refunds/page.tsx','Sending a cancellation refund cancels the order and restores its','Sending a cancellation refund cancels the order and restores unshipped')
# Formatter wraps this paragraph, so retain its exact subsequent text and explain return inspection.
edit(web/'app/(admin)/admin/refunds/page.tsx','reserved stock. After-delivery settlements keep the delivery history;','reserved stock. Dispatched goods need return inspection before restocking. After-delivery settlements keep the delivery history;')
edit(api/'docs/refunds.md','Submission cancels the order and restores reserved stock once.','Submission cancels the order and restores unshipped reserved stock once. Dispatched goods need return inspection before restocking; a refund does not automatically make shipped items sellable.')
# Exercise callback authentication and verify that reserved unknown outcomes cannot enable over-refunds.
p=api/'routes/refund_database_test.go'
edit(p,'cfg.IoTecMobileMoneyEnabled = true','cfg.IoTecMobileMoneyEnabled = true\n cfg.IoTecCallbackSecret = "callback-test"')
edit(p,'\trole = "user"','\tif code,_:=request("POST","/payments/iotec/refund-callback",map[string]string{"externalId":"unknown"});code!=401{t.Fatal("unauthenticated refund callback accepted",code)}\n\trole = "user"')
edit(p,'if available, err := refundableAmount','''
 if code,_:=request("POST","/admin/refunds",makeDraft(500,"0771234567","return_settlement"));code!=409{t.Fatal("submitting refund allowed an additional payout",code)}
 if available, err := refundableAmount''')
