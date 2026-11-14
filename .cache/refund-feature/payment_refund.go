package models

import "time"

// PaymentRefund is an admin-only ledger. Original collections remain immutable.
type PaymentRefund struct {
 ID string `json:"id" gorm:"primaryKey;type:char(36)"`
 OrderID string `json:"orderId" gorm:"index;not null"`
 OrderNumber string `json:"orderNumber"`
 UserID string `json:"userId" gorm:"index"`
 CustomerEmail string `json:"customerEmail"`
 CustomerName string `json:"customerName"`
 PaymentAttemptID string `json:"paymentAttemptId" gorm:"index;not null"`
 OriginalTransactionID string `json:"originalTransactionId" gorm:"index;not null"`
 Phone string `json:"-" gorm:"not null"`
 MaskedPhone string `json:"maskedPhone"`
 Amount int64 `json:"amount" gorm:"not null"`
 Currency string `json:"currency" gorm:"not null"`
 Kind string `json:"kind" gorm:"not null"`
 Reason string `json:"reason" gorm:"type:text;not null"`
 SettlementNotes string `json:"settlementNotes" gorm:"type:text"`
 Status string `json:"status" gorm:"index;not null"`
 ProviderReference string `json:"providerReference" gorm:"index"`
 ProviderStatus string `json:"providerStatus"`
 FailureReason string `json:"failureReason"`
 CreatedBy string `json:"createdBy"`
 SubmittedBy string `json:"submittedBy"`
 SubmittedAt *time.Time `json:"submittedAt"`
 VerifiedAt *time.Time `json:"verifiedAt"`
 CreatedAt time.Time `json:"createdAt"`
 UpdatedAt time.Time `json:"updatedAt"`
}
