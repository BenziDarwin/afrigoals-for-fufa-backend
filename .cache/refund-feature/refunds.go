package routes

import (
 "context"
 "errors"
 "fmt"
 "strings"
 "time"

 "afrigoals-api.com/middleware"
 "afrigoals-api.com/models"
 "afrigoals-api.com/services"
 "github.com/gofiber/fiber/v2"
 "github.com/google/uuid"
 "gorm.io/gorm"
 "gorm.io/gorm/clause"
)

var refundReservations = []string{"draft","submitting","processing","unknown","succeeded"}

func RegisterRefunds(api fiber.Router, auth fiber.Handler, deps Deps) {
 admin := api.Group("/admin/refunds",auth,middleware.RequireAdmin())
 admin.Get("/",func(c *fiber.Ctx) error {
  var rows []models.PaymentRefund
  q := deps.DB.Order("created_at desc").Limit(500)
  if search := strings.TrimSpace(c.Query("search")); search != "" {
   like := "%"+strings.ToLower(search)+"%"
   q=q.Where("LOWER(order_number) LIKE ? OR LOWER(customer_email) LIKE ? OR original_transaction_id = ? OR provider_reference = ?",like,like,search,search)
  }
  if err := q.Find(&rows).Error; err != nil { return refundError(c,500,"Could not load refunds") }
  if rows == nil { rows=[]models.PaymentRefund{} }
  return c.JSON(rows)
 })
 admin.Post("/",func(c *fiber.Ctx) error { return prepareRefund(c,deps) })
 admin.Post("/:id/submit",func(c *fiber.Ctx) error { return submitRefund(c,deps) })
 admin.Post("/:id/reconcile",func(c *fiber.Ctx) error { return reconcileRefund(c,deps) })
 admin.Post("/:id/reject",func(c *fiber.Ctx) error {
  result:=deps.DB.Model(&models.PaymentRefund{}).Where("id = ? AND status = ?",c.Params("id"),"draft").Updates(map[string]any{"status":"rejected","failure_reason":"Draft rejected by administrator","submitted_by":c.Locals("userID")})
  if result.Error != nil { return refundError(c,500,"Could not reject draft") }
  if result.RowsAffected != 1 { return refundError(c,409,"Only an unsubmitted draft can be rejected") }
  return c.JSON(fiber.Map{"ok":true})
 })
}

func refundError(c *fiber.Ctx,status int,msg string) error { return c.Status(status).JSON(fiber.Map{"error":msg}) }

func refundOrderWasDelivered(tx *gorm.DB, order models.Order) (bool,error) {
 if normalizeOrderStatusName(order.Status)=="delivered" { return true,nil }
 var count int64
 if err:=tx.Model(&models.OrderStatusEvent{}).Where("order_id = ? AND LOWER(status) = ?",order.ID,"delivered").Count(&count).Error;err!=nil{return false,err}
 if count>0{return true,nil}
 if err:=tx.Model(&models.OrderDeliveryAssignment{}).Where("order_id = ? AND status = ?",order.ID,"delivered").Count(&count).Error;err!=nil{return false,err}
 return count>0,nil
}

func refundableAmount(tx *gorm.DB,attempt models.PaymentAttempt) (int64,error) {
 var reserved int64
 if err:=tx.Model(&models.PaymentRefund{}).Where("payment_attempt_id = ? AND status IN ?",attempt.ID,refundReservations).Select("COALESCE(SUM(amount),0)").Scan(&reserved).Error;err!=nil{return 0,err}
 return attempt.Amount-reserved,nil
}

func prepareRefund(c *fiber.Ctx,deps Deps) error {
 var req struct {
  TransactionID string `json:"transactionId"`
  Phone string `json:"phone"`
  CustomerName string `json:"customerName"`
  Amount int64 `json:"amount"`
  Kind string `json:"kind"`
  Reason string `json:"reason"`
  SettlementNotes string `json:"settlementNotes"`
 }
 if c.BodyParser(&req)!=nil{return refundError(c,400,"Invalid refund details")}
 req.TransactionID=strings.TrimSpace(req.TransactionID);req.CustomerName=strings.TrimSpace(req.CustomerName);req.Reason=strings.TrimSpace(req.Reason);req.SettlementNotes=strings.TrimSpace(req.SettlementNotes)
 phone,err:=services.NormalizeUgandanMobileNumber(req.Phone)
 if err!=nil{return refundError(c,400,err.Error())}
 if req.Amount<500 || req.Amount>1000000000 || len(req.Reason)<5 || len(req.Reason)>2000 || len(req.CustomerName)<2 || len(req.CustomerName)>150 || len(req.SettlementNotes)>4000{return refundError(c,400,"Provide a customer name, reason and whole UGX amount of at least 500")}
 if req.Kind!="cancellation" && req.Kind!="return_settlement"{return refundError(c,400,"Choose cancellation or return settlement")}
 if req.Kind=="return_settlement" && len(req.SettlementNotes)<10{return refundError(c,400,"Record the agreed return or complaint settlement before preparing a refund")}
 var attempt models.PaymentAttempt
 if err:=deps.DB.Where("provider_reference = ? AND provider = ? AND status = ? AND verified_at IS NOT NULL",req.TransactionID,"iotec","succeeded").First(&attempt).Error;err!=nil{return refundError(c,404,"No verified successful IOTEC payment matches that transaction ID")}
 if deps.IoTec==nil{return refundError(c,503,"IOTEC is not configured")}
 ctx,cancel:=context.WithTimeout(context.Background(),providerCallTimeout);defer cancel()
 originalPhone,err:=deps.IoTec.VerifiedCollectionPayer(ctx,attempt.ProviderReference,services.VerificationSnapshot{Amount:attempt.Amount,Currency:attempt.Currency,MerchantReference:attempt.MerchantReference,ExpectedWalletID:deps.Config.IoTecWalletID})
 if err!=nil{return refundError(c,409,"Could not verify the original payment and payer with IOTEC. Reconcile the payment before refunding.")}
 if phone!=originalPhone{return refundError(c,400,"Refund number must match the original IOTEC payer number")}
 row:=models.PaymentRefund{}
 err=deps.DB.Transaction(func(tx *gorm.DB) error {
  var order models.Order
  if err:=tx.Clauses(clause.Locking{Strength:"UPDATE"}).First(&order,"id = ?",attempt.OrderID).Error;err!=nil{return err}
  if order.UserID!=attempt.UserID || order.PaymentMethod!=mobileMoneyMethod || !strings.EqualFold(attempt.Currency,"UGX"){return fmt.Errorf("payment does not match an eligible UGX mobile money order")}
  delivered,err:=refundOrderWasDelivered(tx,order);if err!=nil{return err}
  if req.Kind=="cancellation" && delivered{return fmt.Errorf("delivered orders require a documented return settlement")}
  if req.Kind=="return_settlement" && !delivered{return fmt.Errorf("use a cancellation refund for an order that has not been delivered")}
  available,err:=refundableAmount(tx,attempt);if err!=nil{return err}
  if req.Amount>available{return fmt.Errorf("amount exceeds the remaining refundable balance (UGX %d), including reserved refunds",available)}
  row=models.PaymentRefund{ID:uuid.NewString(),OrderID:order.ID,OrderNumber:order.OrderNumber,UserID:order.UserID,CustomerEmail:order.Email,CustomerName:req.CustomerName,PaymentAttemptID:attempt.ID,OriginalTransactionID:attempt.ProviderReference,Phone:phone,MaskedPhone:services.MaskPhone(phone),Amount:req.Amount,Currency:"UGX",Kind:req.Kind,Reason:req.Reason,SettlementNotes:req.SettlementNotes,Status:"draft",CreatedBy:c.Locals("userID").(string)}
  return tx.Create(&row).Error
 })
 if err!=nil{return refundError(c,409,err.Error())}
 return c.Status(201).JSON(row)
}

// Claiming and committing happens before the external call. A timeout/crash
// leaves a reserved record which can only be rechecked, never resubmitted.
func claimRefund(db *gorm.DB,id,admin string) (models.PaymentRefund,error) {
 var row models.PaymentRefund
 err:=db.Transaction(func(tx *gorm.DB) error {
  // All writers lock the order first, keeping reservations serialised.
  if err:=tx.First(&row,"id = ?",id).Error;err!=nil{return err}
  var order models.Order
  if err:=tx.Clauses(clause.Locking{Strength:"UPDATE"}).Preload("Items").First(&order,"id = ?",row.OrderID).Error;err!=nil{return err}
  if err:=tx.Clauses(clause.Locking{Strength:"UPDATE"}).First(&row,"id = ?",id).Error;err!=nil{return err}
  if row.Status!="draft"{return fmt.Errorf("refund already submitted or rejected; use Recheck instead")}
  if row.Kind=="cancellation" {
   delivered,err:=refundOrderWasDelivered(tx,order);if err!=nil{return err}
   if delivered{return fmt.Errorf("order has been delivered; reject this draft and record a return settlement")}
   if !isCancelledOrderStatus(order.Status) {
    if err:=restoreOrderStock(tx,order.Items);err!=nil{return err}
    order.Status="cancelled"
    if err:=tx.Model(&order).Update("status","cancelled").Error;err!=nil{return err}
    if err:=tx.Create(&models.OrderStatusEvent{OrderID:order.ID,Status:"cancelled",Note:"Order cancelled for refund "+row.ID}).Error;err!=nil{return err}
   }
  }
  now:=time.Now().UTC();row.Status="submitting";row.SubmittedBy=admin;row.SubmittedAt=&now
  return tx.Save(&row).Error
 })
 return row,err
}

func submitRefund(c *fiber.Ctx,deps Deps) error {
 var req struct { Confirm bool `json:"confirm"` }
 if c.BodyParser(&req)!=nil || !req.Confirm{return refundError(c,400,"Confirm the reviewed refund before sending money")}
 if deps.IoTec==nil || !deps.Config.IoTecMobileMoneyEnabled{return refundError(c,503,"IOTEC payments are not enabled")}
 row,err:=claimRefund(deps.DB,c.Params("id"),c.Locals("userID").(string))
 if err!=nil{return refundError(c,409,err.Error())}
 ctx,cancel:=context.WithTimeout(context.Background(),providerCallTimeout);defer cancel()
 _,err=deps.IoTec.DisburseRefund(ctx,row.ID,row.Phone,row.CustomerName,row.Amount)
 if err!=nil {
  status:="unknown";reason:="Submission outcome unknown. Recheck this refund; do not create another payout."
  if errors.Is(err,services.ErrIoTecRejected){status="failed";reason="IOTEC rejected this refund. Review wallet permissions, balance and recipient details."}
  if saveErr:=deps.DB.Model(&models.PaymentRefund{}).Where("id = ? AND status = ?",row.ID,"submitting").Updates(map[string]any{"status":status,"failure_reason":reason}).Error;saveErr!=nil{return refundError(c,500,"Refund was submitted but its result could not be saved. Recheck; do not retry.")}
  deps.DB.First(&row,"id = ?",row.ID)
  return c.JSON(row)
 }
 // Even a successful POST response is independently fetched before completion.
 return reconcileRefundID(c,deps,row.ID)
}

func reconcileRefund(c *fiber.Ctx,deps Deps) error {return reconcileRefundID(c,deps,c.Params("id"))}
func reconcileRefundID(c *fiber.Ctx,deps Deps,id string) error {
 var row models.PaymentRefund
 if deps.DB.First(&row,"id = ?",id).Error!=nil{return refundError(c,404,"Refund not found")}
 if row.Status=="draft" || row.Status=="rejected"{return refundError(c,409,"This refund has not been submitted")}
 if row.Status=="succeeded"{return c.JSON(row)}
 if deps.IoTec==nil{return refundError(c,503,"IOTEC is not configured")}
 ctx,cancel:=context.WithTimeout(context.Background(),providerCallTimeout);defer cancel()
 response,err:=deps.IoTec.RefundStatus(ctx,row.ID)
 if err!=nil{return refundError(c,502,"IOTEC could not confirm this refund. Its reservation remains in place; recheck later.")}
 status,reason:=services.VerifyRefundResponse(response,row.ID,deps.Config.IoTecWalletID,row.Phone,row.Amount)
 err=deps.DB.Transaction(func(tx *gorm.DB) error {
  var order models.Order
  if err:=tx.Clauses(clause.Locking{Strength:"UPDATE"}).First(&order,"id = ?",row.OrderID).Error;err!=nil{return err}
  if err:=tx.Clauses(clause.Locking{Strength:"UPDATE"}).First(&row,"id = ?",row.ID).Error;err!=nil{return err}
  if row.Status=="succeeded"{return nil}
  if row.ProviderReference!="" && row.ProviderReference!=response.TrackingID{status="unknown";reason="Refund transaction reference changed; manual review required"}
  now:=time.Now().UTC()
  row.Status=status;row.FailureReason=reason;row.ProviderStatus=response.RawStatus
  if status!="unknown"{row.ProviderReference=response.TrackingID;row.VerifiedAt=&now}
  if err:=tx.Save(&row).Error;err!=nil{return err}
  if status=="succeeded" {
   return tx.Create(&models.OrderStatusEvent{OrderID:order.ID,Status:order.Status,Note:fmt.Sprintf("IOTEC refund verified: UGX %d; refund %s; transaction %s",row.Amount,row.ID,row.ProviderReference)}).Error
  }
  return nil
 })
 if err!=nil{return refundError(c,500,"Could not save refund verification. Recheck this refund.")}
 return c.JSON(row)
}
