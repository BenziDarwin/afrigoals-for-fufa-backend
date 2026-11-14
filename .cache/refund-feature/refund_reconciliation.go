package services

import (
 "afrigoals-api.com/models"
 "gorm.io/gorm"
)

// RefundCashflow uses provider-verification dates. Gross collections include
// cancelled orders: cancelling an order does not itself return collected money.
type RefundCashflow struct {
 Collections int64 `json:"collections"`
 RefundsPaid int64 `json:"refundsPaid"`
 RefundsReserved int64 `json:"refundsReserved"`
 NetCollections int64 `json:"netCollections"`
}

func BuildRefundCashflow(db *gorm.DB,filter ReconciliationFilter) (RefundCashflow,error) {
 result:=RefundCashflow{}
 scope:=func(q *gorm.DB,column string)*gorm.DB{
  if !filter.From.IsZero(){q=q.Where(column+" >= ?",filter.From.In(kampalaLocation()))}
  if !filter.To.IsZero(){q=q.Where(column+" < ?",filter.To.In(kampalaLocation()).AddDate(0,0,1))}
  return q
 }
 if err:=scope(db.Model(&models.PaymentAttempt{}).Where("provider = ? AND status = ? AND verified_at IS NOT NULL","iotec","succeeded"),"verified_at").Select("COALESCE(SUM(amount),0)").Scan(&result.Collections).Error;err!=nil{return result,err}
 if err:=scope(db.Model(&models.PaymentRefund{}).Where("status = ?","succeeded"),"verified_at").Select("COALESCE(SUM(amount),0)").Scan(&result.RefundsPaid).Error;err!=nil{return result,err}
 if err:=scope(db.Model(&models.PaymentRefund{}).Where("status IN ?",[]string{"draft","submitting","processing","unknown"}),"created_at").Select("COALESCE(SUM(amount),0)").Scan(&result.RefundsReserved).Error;err!=nil{return result,err}
 result.NetCollections=result.Collections-result.RefundsPaid
 return result,nil
}
