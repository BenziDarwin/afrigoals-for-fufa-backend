package services

import (
 "context"
 "encoding/json"
 "fmt"
 "net/http"
 "net/url"
 "strings"
)

type IoTecRefundResponse struct {
 IoTecCollectionResponse
 Payee string
}

// VerifiedCollectionPayer requires an exact provider-side match, not just the
// three visible digits stored in the checkout ledger.
func (c *IoTecClient) VerifiedCollectionPayer(ctx context.Context, reference string, snap VerificationSnapshot) (string, error) {
 raw, code, err := c.do(ctx, http.MethodGet, "/api/collections/status/"+url.PathEscape(reference), nil)
 if err != nil { return "", err }
 response, err := c.interpret(raw, code)
 if err != nil { return "", err }
 if response.TrackingID != reference || response.ExternalID != snap.MerchantReference || response.WalletID != snap.ExpectedWalletID || !strings.EqualFold(response.Currency,snap.Currency) || !response.AmountPresent || response.Amount != float64(snap.Amount) || response.Status != IoTecStatusSucceeded {
  return "", fmt.Errorf("original payment could not be verified against this order")
 }
 var fields struct { Payer string `json:"payer"` }
 if err := json.Unmarshal(raw,&fields); err != nil { return "", err }
 phone, err := NormalizeUgandanMobileNumber(fields.Payer)
 if err != nil { return "", fmt.Errorf("provider did not return a verifiable original payer number; contact IOTEC for review") }
 return phone,nil
}

func (c *IoTecClient) DisburseRefund(ctx context.Context, reference, phone, name string, amount int64) (IoTecRefundResponse,error) {
 raw, code, err := c.do(ctx,http.MethodPost,"/api/disbursements/disburse",map[string]any{
  "category":"MobileMoney", "currency":"UGX", "walletId":c.cfg.IoTecWalletID,
  "externalId":reference, "payee":phone,"payeeName":name,"amount":amount,
  "payerNote":"Afrigoals refund", "payeeNote":"Afrigoals order refund",
 })
 if err != nil { return IoTecRefundResponse{},err }
 // Throttling/timeouts are ambiguous; never automatically resend a payout.
 if code == http.StatusRequestTimeout || code == http.StatusTooManyRequests { return IoTecRefundResponse{},ErrIoTecUnavailable }
 return c.interpretRefund(raw,code)
}

func (c *IoTecClient) RefundStatus(ctx context.Context, externalID string) (IoTecRefundResponse,error) {
 raw,code,err := c.do(ctx,http.MethodGet,"/api/disbursements/external-id/"+url.PathEscape(externalID),nil)
 if err != nil { return IoTecRefundResponse{},err }
 return c.interpretRefund(raw,code)
}

func (c *IoTecClient) interpretRefund(raw []byte,code int) (IoTecRefundResponse,error) {
 response,err := c.interpret(raw,code)
 if err != nil { return IoTecRefundResponse{},err }
 var fields struct { Payee string `json:"payee"` }
 if err := json.Unmarshal(raw,&fields); err != nil { return IoTecRefundResponse{},ErrIoTecUnavailable }
 return IoTecRefundResponse{IoTecCollectionResponse:response,Payee:fields.Payee},nil
}

// A successful-looking response with incomplete or conflicting details must
// stay unknown and keep its reservation, so a second payout cannot follow it.
func VerifyRefundResponse(resp IoTecRefundResponse, reference, wallet, phone string, amount int64) (string,string) {
 payee,err := NormalizeUgandanMobileNumber(resp.Payee)
 if err != nil || payee != phone || resp.TrackingID == "" || resp.ExternalID != reference || resp.WalletID != wallet || !strings.EqualFold(resp.Currency,"UGX") || !resp.AmountPresent || resp.Amount != float64(amount) {
  return "unknown","Refund response requires reconciliation: transaction details do not match"
 }
 switch resp.Status {
 case IoTecStatusSucceeded: return "succeeded",""
 case IoTecStatusFailed, IoTecStatusCancelled: return "failed","IOTEC reported that the refund did not complete"
 default: return "processing","Awaiting IOTEC confirmation"
 }
}
