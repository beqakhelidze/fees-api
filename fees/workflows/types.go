package workflow

import "time"

// BillInput starts a bill workflow for a billing period.
type BillInput struct {
	ID          string    `json:"id"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
}

// Update names registered on BillWorkflow.
const (
	UpdateAddLineItem = "AddLineItem"
	UpdateCloseBill   = "CloseBill"
	QueryGetBill      = "GetBill"
)
