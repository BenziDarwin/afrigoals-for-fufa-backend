from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api')
def edit(p,a,b):
 s=p.read_text(encoding='utf-8');assert a in s,(str(p),a[:90]);p.write_text(s.replace(a,b),encoding='utf-8')
# Contact edits must not save stale payment/order statuses over a refund claim.
edit(api/'routes/orders.go','if err := deps.DB.Save(&order).Error; err != nil {','if err := deps.DB.Model(&order).Updates(map[string]any{"delivery_contact_name":contactName,"delivery_contact_phone":contactPhone,"delivery_alt_phone":altPhone,"delivery_note":note}).Error; err != nil {')
# Current status can be edited; a historical delivery timestamp is conclusive.
edit(api/'routes/refunds.go','"order_id = ? AND status = ?", order.ID, "delivered"','"order_id = ? AND (status = ? OR delivered_at IS NOT NULL)", order.ID, "delivered"')
edit(api/'services/iotec_refunds.go','return IoTecRefundResponse{IoTecCollectionResponse: response, Payee: fields.Payee}, nil','if strings.EqualFold(response.RawStatus, "RolledBack") { response.Status = IoTecStatusFailed }\n\treturn IoTecRefundResponse{IoTecCollectionResponse: response, Payee: fields.Payee}, nil')
# Add authenticated disbursement callbacks without exposing the refund ledger.
p=api/'routes/refunds.go'
edit(p,'\tadmin := api.Group("/admin/refunds", auth, middleware.RequireAdmin())','''
 api.Post("/payments/iotec/refund-callback", func(c *fiber.Ctx) error {
  if !services.VerifyIoTecCallbackSecret(deps.Config.IoTecCallbackSecret, c.Get(services.IoTecCallbackHeader())) {
   return refundError(c, 401, "Invalid callback credential")
  }
  var payload struct { ExternalID string `json:"externalId"` }
  if c.BodyParser(&payload) != nil || payload.ExternalID == "" { return refundError(c, 400, "Missing refund reference") }
  if err := reconcileRefundID(c, deps, payload.ExternalID); err != nil { return err }
  if c.Response().StatusCode() >= 400 { return nil }
  return c.JSON(fiber.Map{"ok":true})
 })
\tadmin := api.Group("/admin/refunds", auth, middleware.RequireAdmin())''')
# Expand database assertions to include actual stock restoration and balances.
p=api/'routes/refund_database_test.go'
edit(p,'"gorm.io/driver/postgres"','"gorm.io/driver/postgres"\n"gorm.io/datatypes"')
edit(p,'\tattempt := models.PaymentAttempt{','''
 product:=models.Product{Name:"Refund test item",Slug:"refund-test-item",Price:1700,Currency:"UGX",Stock:5,Images:datatypes.JSON("[]"),Variants:datatypes.JSON("[]")}
 if err:=tx.Create(&product).Error;err!=nil{t.Fatal(err)}
 if err:=tx.Create(&models.OrderItem{OrderID:order.ID,ProductID:product.ID,ProductName:product.Name,Quantity:1,PriceAtPurchase:1700,LineSubtotal:1700}).Error;err!=nil{t.Fatal(err)}
\tattempt := models.PaymentAttempt{''')
edit(p,'\tflow, err := services.BuildRefundCashflow','''
 if err:=tx.First(&product,"id = ?",product.ID).Error;err!=nil{t.Fatal(err)}
 if product.Stock!=6{t.Fatal("cancelled stock not restored exactly once",product.Stock)}
 summaries:=[]models.Order{order}
 if err:=services.AttachRefundSummaries(tx,summaries);err!=nil || summaries[0].RefundedAmount!=1200{t.Fatal("refund not visible on order",err)}
\tflow, err := services.BuildRefundCashflow''')
edit(p,'if _, err = refundableAmount(tx, attempt); err != nil {\n\t\tt.Fatal(err)\n\t}','if available, err := refundableAmount(tx, attempt); err != nil || available != 0 { t.Fatal("unknown/submitting refund lost reservation",available,err) }')
edit(api/'docs/refunds.md','Use Recheck until provider confirmation arrives.','Use Recheck until provider confirmation arrives. For automatic provider notifications, configure the IOTEC wallet disbursement callback as `/api/v1/payments/iotec/refund-callback` with `X-IoTec-Callback-Secret` matching the configured collection callback secret; callbacks independently verify the payout with IOTEC before changing the ledger.')
with (api/'services/iotec_refunds_test.go').open('a',encoding='utf-8') as f:f.write('''
func TestRefundUnavailableNeverRetriesPayout(t *testing.T) {
 for _, code := range []int{408,429,503} {
  t.Run(fmt.Sprint(code),func(t *testing.T){
   posts:=0
   server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
    if r.URL.Path=="/token"{fmt.Fprint(w,`{"access_token":"mock","expires_in":300}`);return}
    posts++;w.WriteHeader(code);fmt.Fprint(w,`{"message":"unknown outcome"}`)
   }));defer server.Close()
   client:=NewIoTecClient(Config{IoTecBaseURL:server.URL,IoTecTokenURL:server.URL+"/token",IoTecClientID:"id",IoTecClientSecret:"secret",IoTecWalletID:"wallet"})
   _,err:=client.DisburseRefund(context.Background(),"case","256771234567","Customer",800)
   if !errors.Is(err,ErrIoTecUnavailable)||posts!=1{t.Fatal("unsafe retry or error classification",posts,err)}
  })
 }
}
''')
edit(api/'services/iotec_refunds_test.go','"context"','"context"\n"errors"\n"fmt"')
