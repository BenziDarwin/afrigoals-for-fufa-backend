package services

import (
 "afrigoals-api.com/models"
 "gorm.io/gorm"
)

// Only aggregate amounts leave the admin refund ledger. Customer details,
// phone numbers and internal settlement notes are never included in orders.
func AttachRefundSummaries(db *gorm.DB,orders []models.Order) error {
 if len(orders)==0{return nil}
 ids:=make([]string,0,len(orders));for _,order:=range orders{ids=append(ids,order.ID)}
 var rows []struct{OrderID string; RefundedAmount int64; RefundPendingAmount int64}
 if err:=db.Model(&models.PaymentRefund{}).Select("order_id, SUM(CASE WHEN status = 'succeeded' THEN amount ELSE 0 END) AS refunded_amount, SUM(CASE WHEN status IN ('draft','submitting','processing','unknown') THEN amount ELSE 0 END) AS refund_pending_amount").Where("order_id IN ?",ids).Group("order_id").Scan(&rows).Error;err!=nil{return err}
 for i:=range orders{for _,row:=range rows{if orders[i].ID==row.OrderID{orders[i].RefundedAmount=row.RefundedAmount;orders[i].RefundPendingAmount=row.RefundPendingAmount}}}
 return nil
}
