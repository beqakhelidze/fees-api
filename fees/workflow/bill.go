package workflow

import (
	"fmt"
	"strings"
	"time"

	"encore.app/fees/domain"
	"go.temporal.io/sdk/workflow"
)

// BillWorkflow owns the lifecycle of a single fee bill for a billing period.
// Mutations go through Temporal Updates so concurrent adds are serialized and
// closed bills reliably reject new line items.
func BillWorkflow(ctx workflow.Context, input BillInput) (domain.Bill, error) {
	state := domain.Bill{
		ID:          input.ID,
		Status:      domain.BillStatusOpen,
		PeriodStart: input.PeriodStart,
		PeriodEnd:   input.PeriodEnd,
		LineItems:   []domain.LineItem{},
		Totals:      []domain.Money{},
	}

	if err := workflow.SetQueryHandler(ctx, QueryGetBill, func() (domain.Bill, error) {
		return state, nil
	}); err != nil {
		return state, err
	}

	if err := workflow.SetUpdateHandlerWithOptions(ctx, UpdateAddLineItem,
		func(ctx workflow.Context, item domain.LineItem) (domain.Bill, error) {
			item.Currency = domain.NormalizeCurrency(item.Currency)
			state.LineItems = append(state.LineItems, item)
			state.Totals = domain.TotalsFromLineItems(state.LineItems)
			return state, nil
		},
		workflow.UpdateHandlerOptions{
			Validator: func(ctx workflow.Context, item domain.LineItem) error {
				if state.Status != domain.BillStatusOpen {
					return fmt.Errorf("bill is closed")
				}
				if strings.TrimSpace(item.ID) == "" {
					return fmt.Errorf("line item id is required")
				}
				if strings.TrimSpace(item.Description) == "" {
					return fmt.Errorf("line item description is required")
				}
				if item.AmountMinor <= 0 {
					return fmt.Errorf("amount_minor must be positive")
				}
				c := domain.NormalizeCurrency(item.Currency)
				if c != domain.CurrencyUSD && c != domain.CurrencyGEL {
					return fmt.Errorf("currency must be USD or GEL")
				}
				for _, existing := range state.LineItems {
					if existing.ID == item.ID {
						return fmt.Errorf("line item %q already exists", item.ID)
					}
				}
				return nil
			},
		},
	); err != nil {
		return state, err
	}

	closedCh := workflow.NewChannel(ctx)

	if err := workflow.SetUpdateHandlerWithOptions(ctx, UpdateCloseBill,
		func(ctx workflow.Context, _ struct{}) (domain.Bill, error) {
			now := workflow.Now(ctx).UTC()
			state.Status = domain.BillStatusClosed
			state.ClosedAt = &now
			state.Totals = domain.TotalsFromLineItems(state.LineItems)
			closedCh.Send(ctx, true)
			return state, nil
		},
		workflow.UpdateHandlerOptions{
			Validator: func(ctx workflow.Context, _ struct{}) error {
				if state.Status != domain.BillStatusOpen {
					return fmt.Errorf("bill is already closed")
				}
				return nil
			},
		},
	); err != nil {
		return state, err
	}

	delay := state.PeriodEnd.Sub(workflow.Now(ctx))
	if delay < 0 {
		delay = 0
	}

	selector := workflow.NewSelector(ctx)
	manualClose := false
	periodEnded := false

	selector.AddReceive(closedCh, func(c workflow.ReceiveChannel, _ bool) {
		var v bool
		c.Receive(ctx, &v)
		manualClose = true
	})
	selector.AddFuture(workflow.NewTimer(ctx, delay), func(f workflow.Future) {
		_ = f.Get(ctx, nil)
		periodEnded = true
	})

	for !manualClose && !periodEnded {
		selector.Select(ctx)
	}

	if state.Status == domain.BillStatusOpen {
		now := workflow.Now(ctx).UTC()
		state.Status = domain.BillStatusClosed
		state.ClosedAt = &now
		state.Totals = domain.TotalsFromLineItems(state.LineItems)
	}

	// Persist final invoice (covers manual close and period-end auto-close).
	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	})
	if err := workflow.ExecuteActivity(actCtx, "PersistBill", state).Get(ctx, nil); err != nil {
		return state, err
	}

	return state, nil
}

// WorkflowID returns the Temporal workflow id for a bill.
func WorkflowID(billID string) string {
	return "bill/" + billID
}
