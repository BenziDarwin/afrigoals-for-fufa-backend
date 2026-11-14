package models

import "time"

// A customer request is reviewable evidence, never authority to send money.
type RefundRequest struct {
 ID string `json:"id" gorm:"primaryKey;type:char(36)"`
 OrderID string `json:"orderId" gorm:"index;not null"`
 OrderNumber string `json:"orderNumber"`
 UserID string `json:"userId" gorm:"index;not null"`
 CustomerEmail string `json:"customerEmail"`
 CustomerName string `json:"customerName"`
 Phone string `json:"-"`
 MaskedPhone string `json:"maskedPhone"`
 PaymentAttemptID string `json:"paymentAttemptId" gorm:"index;not null"`
 OriginalTransactionID string `json:"originalTransactionId"`
 ReceiptReference string `json:"receiptReference"`
 Amount int64 `json:"amount"`
 Kind string `json:"kind"`
 Reason string `json:"reason" gorm:"type:text"`
 Details string `json:"details" gorm:"type:text"`
 Status string `json:"status" gorm:"index;not null"`
 ReviewNote string `json:"reviewNote" gorm:"type:text"`
 ReviewedBy string `json:"-"`
 ReviewedAt *time.Time `json:"reviewedAt"`
 RefundID string `json:"refundId" gorm:"index"`
 CreatedAt time.Time `json:"createdAt"`
 UpdatedAt time.Time `json:"updatedAt"`
}
