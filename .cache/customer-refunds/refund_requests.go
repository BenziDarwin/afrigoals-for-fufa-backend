package routes

import (
 "context"
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

// Public review messages are deliberately separate from payout failure details
// and internal settlement notes in the admin refund ledger.
type refundRequestView struct {
 models.RefundRequest
 RefundStatus string `json:"refundStatus"`
 ApprovedAmount int64 `json:"approvedAmount"`
 RefundTransactionID string `json:"refundTransactionId"`
}
type adminRefundRequestView struct {
 refundRequestView
 Phone string `json:"phone"`
 OrderStatus string `json:"orderStatus"`
}

func RegisterRefundRequests(api fiber.Router,auth fiber.Handler,deps Deps) {
 api.Get("/orders/:id/refund-requests",auth,func(c *fiber.Ctx)error{return customerRefundRequests(c,deps)})
 api.Post("/orders/:id/refund-requests",auth,func(c *fiber.Ctx)error{return createRefundRequest(c,deps)})
 admin:=api.Group("/admin/refund-requests",auth,middleware.RequireAdmin())
 admin.Get("/",func(c *fiber.Ctx)error{
  var requests []models.RefundRequest
  q:=deps.DB.Order("created_at asc").Limit(500)
  if status:=c.Query("status");status!=""{
   if status!="requested"&&status!="approved"&&status!="declined"{return refundError(c,400,"Invalid review status")}
   q=q.Where("status = ?",status)
  }
  if err:=q.Find(&requests).Error;err!=nil{return refundError(c,500,"Could not load refund requests")}
  views,err:=requestViews(deps.DB,requests);if err!=nil{return refundError(c,500,"Could not load refund outcomes")}
  rows:=make([]adminRefundRequestView,0,len(views))
  for _,view:=range views{
   var order models.Order
   if err:=deps.DB.Select("id","status").First(&order,"id = ?",view.OrderID).Error;err!=nil{return refundError(c,500,"Could not load order status")}
   rows=append(rows,adminRefundRequestView{refundRequestView:view,Phone:view.Phone,OrderStatus:order.Status})
  }
  return c.JSON(rows)
 })
 admin.Post("/:id/approve",func(c *fiber.Ctx)error{return approveCustomerRefund(c,deps)})
 admin.Post("/:id/decline",func(c *fiber.Ctx)error{
  var body struct{ReviewNote string `json:"reviewNote"`}
  if c.BodyParser(&body)!=nil{return refundError(c,400,"Invalid review")}
  body.ReviewNote=strings.TrimSpace(body.ReviewNote)
  if len(body.ReviewNote)<5||len(body.ReviewNote)>4000{return refundError(c,400,"Provide a reason for the customer (5–4000 characters)")}
  now:=time.Now().UTC()
  result:=deps.DB.Model(&models.RefundRequest{}).Where("id = ? AND status = ?",c.Params("id"),"requested").Updates(map[string]any{"status":"declined","review_note":body.ReviewNote,"reviewed_by":c.Locals("userID"),"reviewed_at":now})
  if result.Error!=nil{return refundError(c,500,"Could not save review")}
  if result.RowsAffected!=1{return refundError(c,409,"This request has already been reviewed or does not exist")}
  return c.JSON(fiber.Map{"ok":true})
 })
}

func requestViews(db *gorm.DB,requests []models.RefundRequest)([]refundRequestView,error){
 result:=make([]refundRequestView,0,len(requests));ids:=[]string{}
 for _,r:=range requests{if r.RefundID!=""{ids=append(ids,r.RefundID)}}
 refunds:=[]models.PaymentRefund{}
 if len(ids)>0{if err:=db.Where("id IN ?",ids).Find(&refunds).Error;err!=nil{return nil,err}}
 for _,r:=range requests{
  view:=refundRequestView{RefundRequest:r}
  for _,refund:=range refunds{if refund.ID==r.RefundID{view.RefundStatus=refund.Status;view.ApprovedAmount=refund.Amount;if refund.Status=="succeeded"{view.RefundTransactionID=refund.ProviderReference}}}
  result=append(result,view)
 }
 return result,nil
}

func openRefundRequest(tx *gorm.DB,orderID string)(bool,error){
 var count int64
 err:=tx.Table("refund_requests AS r").Joins("LEFT JOIN payment_refunds p ON p.id = r.refund_id").Where("r.order_id = ? AND (r.status = 'requested' OR (r.status = 'approved' AND (p.id IS NULL OR p.status IN ?)))",orderID,[]string{"draft","submitting","processing","unknown"}).Count(&count).Error
 return count>0,err
}

func ownRefundOrder(c *fiber.Ctx,db *gorm.DB)(models.Order,error){
 userID,_:=c.Locals("userID").(string)
 if userID==""{return models.Order{},fmt.Errorf("unauthorized")}
 var order models.Order
 err:=db.Where("id = ? AND user_id = ?",c.Params("id"),userID).First(&order).Error
 return order,err
}

func customerRefundRequests(c *fiber.Ctx,deps Deps)error{
 order,err:=ownRefundOrder(c,deps.DB);if err!=nil{return refundError(c,404,"Order not found")}
 var requests []models.RefundRequest
 if err:=deps.DB.Where("order_id = ? AND user_id = ?",order.ID,order.UserID).Order("created_at desc").Find(&requests).Error;err!=nil{return refundError(c,500,"Could not load requests")}
 views,err:=requestViews(deps.DB,requests);if err!=nil{return refundError(c,500,"Could not load request status")}
 var attempts []models.PaymentAttempt
 if err:=deps.DB.Where("order_id = ? AND user_id = ? AND provider = ? AND status = ? AND verified_at IS NOT NULL",order.ID,order.UserID,"iotec","succeeded").Order("created_at desc").Find(&attempts).Error;err!=nil{return refundError(c,500,"Could not load eligible payments")}
 payments:=[]fiber.Map{}
 for _,attempt:=range attempts{
  if order.PaymentMethod!=mobileMoneyMethod||!strings.EqualFold(attempt.Currency,"UGX"){continue}
  available,err:=refundableAmount(deps.DB,attempt);if err!=nil{return refundError(c,500,"Could not calculate refundable balance")}
  if available>=500{payments=append(payments,fiber.Map{"transactionId":attempt.ProviderReference,"maskedPhone":attempt.MaskedPhone,"available":available})}
 }
 delivered,err:=refundOrderWasDelivered(deps.DB,order);if err!=nil{return refundError(c,500,"Could not check delivery status")}
 open,err:=openRefundRequest(deps.DB,order.ID);if err!=nil{return refundError(c,500,"Could not check pending requests")}
 return c.JSON(fiber.Map{"items":views,"payments":payments,"delivered":delivered,"hasOpenRequest":open})
}

func createRefundRequest(c *fiber.Ctx,deps Deps)error{
 role,_:=c.Locals("role").(string);if isAdminRole(role){return refundError(c,403,"Use the admin refund review process")}
 // Check ownership before looking up transaction IDs or returning payment details.
 order,err:=ownRefundOrder(c,deps.DB);if err!=nil{return refundError(c,404,"Order not found")}
 var body struct{
  TransactionID string `json:"transactionId"`
  CustomerName string `json:"customerName"`
  Phone string `json:"phone"`
  Amount int64 `json:"amount"`
  Reason string `json:"reason"`
  Details string `json:"details"`
  ReceiptReference string `json:"receiptReference"`
 }
 if c.BodyParser(&body)!=nil{return refundError(c,400,"Invalid refund request")}
 body.TransactionID=strings.TrimSpace(body.TransactionID);body.CustomerName=strings.TrimSpace(body.CustomerName);body.Reason=strings.TrimSpace(body.Reason);body.Details=strings.TrimSpace(body.Details);body.ReceiptReference=strings.TrimSpace(body.ReceiptReference)
 phone,err:=services.NormalizeUgandanMobileNumber(body.Phone);if err!=nil{return refundError(c,400,err.Error())}
 if body.Amount<500||body.Amount>1000000000||len(body.CustomerName)<2||len(body.CustomerName)>150||len(body.Reason)<5||len(body.Reason)>2000||len(body.Details)>4000||len(body.ReceiptReference)>150{return refundError(c,400,"Provide your name, original payer number, reason and a whole UGX amount of at least 500")}
 var row models.RefundRequest
 err=deps.DB.Transaction(func(tx *gorm.DB)error{
  if err:=tx.Clauses(clause.Locking{Strength:"UPDATE"}).First(&order,"id = ?",order.ID).Error;err!=nil{return err}
  if order.PaymentMethod!=mobileMoneyMethod{return fmt.Errorf("refund requests currently require a verified IOTEC mobile money payment")}
  open,err:=openRefundRequest(tx,order.ID);if err!=nil{return err};if open{return fmt.Errorf("an active refund request already exists for this order")}
  var attempt models.PaymentAttempt
  if err:=tx.Where("order_id = ? AND user_id = ? AND provider_reference = ? AND provider = ? AND status = ? AND verified_at IS NOT NULL",order.ID,order.UserID,body.TransactionID,"iotec","succeeded").First(&attempt).Error;err!=nil{return fmt.Errorf("choose a verified payment belonging to this order")}
  if !strings.EqualFold(attempt.Currency,"UGX"){return fmt.Errorf("only UGX payments are supported")}
  available,err:=refundableAmount(tx,attempt);if err!=nil{return err};if body.Amount>available{return fmt.Errorf("requested amount exceeds the remaining refundable balance (UGX %d)",available)}
  delivered,err:=refundOrderWasDelivered(tx,order);if err!=nil{return err};kind:="cancellation";if delivered{kind="return_settlement";if len(body.Details)<10{return fmt.Errorf("describe the product issue and requested return or settlement")}}
  row=models.RefundRequest{ID:uuid.NewString(),OrderID:order.ID,OrderNumber:order.OrderNumber,UserID:order.UserID,CustomerEmail:order.Email,CustomerName:body.CustomerName,Phone:phone,MaskedPhone:services.MaskPhone(phone),PaymentAttemptID:attempt.ID,OriginalTransactionID:attempt.ProviderReference,ReceiptReference:body.ReceiptReference,Amount:body.Amount,Kind:kind,Reason:body.Reason,Details:body.Details,Status:"requested"}
  return tx.Create(&row).Error
 })
 if err!=nil{return refundError(c,409,err.Error())}
 return c.Status(201).JSON(refundRequestView{RefundRequest:row})
}

func approveCustomerRefund(c *fiber.Ctx,deps Deps)error{
 var body struct{Amount int64 `json:"amount"`;Kind string `json:"kind"`;ReviewNote string `json:"reviewNote"`}
 if c.BodyParser(&body)!=nil{return refundError(c,400,"Invalid approval")}
 body.ReviewNote=strings.TrimSpace(body.ReviewNote)
 if body.Amount<500||len(body.ReviewNote)<5||len(body.ReviewNote)>4000||(body.Kind!="cancellation"&&body.Kind!="return_settlement"){return refundError(c,400,"Provide an approved amount, refund type and a review message for the customer")}
 if body.Kind=="return_settlement"&&len(body.ReviewNote)<10{return refundError(c,400,"Record the agreed return settlement (at least 10 characters)")}
 var request models.RefundRequest
 if deps.DB.First(&request,"id = ?",c.Params("id")).Error!=nil{return refundError(c,404,"Refund request not found")}
 if request.Status!="requested"{return refundError(c,409,"This request has already been reviewed")}
 var attempt models.PaymentAttempt
 if deps.DB.Where("id = ? AND status = ? AND verified_at IS NOT NULL",request.PaymentAttemptID,"succeeded").First(&attempt).Error!=nil{return refundError(c,409,"Original payment needs reconciliation")}
 if deps.IoTec==nil{return refundError(c,503,"IOTEC is not configured")}
 ctx,cancel:=context.WithTimeout(context.Background(),providerCallTimeout);defer cancel()
 phone,err:=deps.IoTec.VerifiedCollectionPayer(ctx,attempt.ProviderReference,services.VerificationSnapshot{Amount:attempt.Amount,Currency:attempt.Currency,MerchantReference:attempt.MerchantReference,ExpectedWalletID:deps.Config.IoTecWalletID})
 if err!=nil{return refundError(c,409,"Could not verify the original payment with IOTEC; no refund was approved")}
 if phone!=request.Phone{return refundError(c,409,"The supplied phone number does not match the original payer. Decline with a correction request; no refund was approved.")}
 var refund models.PaymentRefund
 err=deps.DB.Transaction(func(tx *gorm.DB)error{
  var order models.Order
  if err:=tx.Clauses(clause.Locking{Strength:"UPDATE"}).First(&order,"id = ?",request.OrderID).Error;err!=nil{return err}
  if err:=tx.Clauses(clause.Locking{Strength:"UPDATE"}).First(&request,"id = ?",request.ID).Error;err!=nil{return err}
  if request.Status!="requested"{return fmt.Errorf("this request has already been reviewed")}
  if request.UserID!=order.UserID||attempt.UserID!=order.UserID||attempt.OrderID!=order.ID||attempt.ProviderReference!=request.OriginalTransactionID||order.PaymentMethod!=mobileMoneyMethod||!strings.EqualFold(attempt.Currency,"UGX"){return fmt.Errorf("original payment does not match this request")}
  available,err:=refundableAmount(tx,attempt);if err!=nil{return err}
  if body.Amount>request.Amount||body.Amount>available{return fmt.Errorf("approved amount exceeds the customer request or remaining refundable balance (UGX %d)",available)}
  delivered,err:=refundOrderWasDelivered(tx,order);if err!=nil{return err}
  if (delivered&&body.Kind!="return_settlement")||(!delivered&&body.Kind!="cancellation"){return fmt.Errorf("delivery status changed; review the correct cancellation or return settlement type")}
  admin,_:=c.Locals("userID").(string);now:=time.Now().UTC();requestID:=request.ID
  refund=models.PaymentRefund{ID:uuid.NewString(),RefundRequestID:&requestID,OrderID:order.ID,OrderNumber:order.OrderNumber,UserID:order.UserID,CustomerEmail:order.Email,CustomerName:request.CustomerName,PaymentAttemptID:attempt.ID,OriginalTransactionID:attempt.ProviderReference,Phone:phone,MaskedPhone:services.MaskPhone(phone),Amount:body.Amount,Currency:"UGX",Kind:body.Kind,Reason:request.Reason,SettlementNotes:body.ReviewNote,Status:"draft",CreatedBy:admin}
  if err:=tx.Create(&refund).Error;err!=nil{return err}
  request.Status="approved";request.ReviewNote=body.ReviewNote;request.ReviewedBy=admin;request.ReviewedAt=&now;request.RefundID=refund.ID
  return tx.Save(&request).Error
 })
 if err!=nil{return refundError(c,409,err.Error())}
 return c.JSON(refund)
}
