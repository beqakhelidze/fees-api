package workflows

import (
	"fmt"
	"strings"
	"time"

	"encore.app/fees/domain"
	"go.temporal.io/sdk/workflow"
)

func BillWorkflow(ctx workflow.Context, input BillInput) (domain.Bill, error) {
	state := domain.Bill{
		ID:          input.ID,
		Status:      domain.BillStatusOpen,
		PeriodStart: input.PeriodStart,
		PeriodEnd:   input.PeriodEnd,
		LineItems:   []domain.LineItem{},
	}

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	})

	billSaved := false

	waitForBill := func(ctx workflow.Context) error {
		return workflow.Await(ctx, func() bool { return billSaved })
	}

	if err := workflow.SetUpdateHandlerWithOptions(ctx, UpdateAddLineItem,
		func(ctx workflow.Context, item domain.LineItem) (domain.Bill, error) {
			item.Currency = domain.NormalizeCurrency(item.Currency)
			state.LineItems = append(state.LineItems, item)

			if err := waitForBill(ctx); err != nil {
				return state, err
			}

			if err := workflow.ExecuteActivity(actCtx, "InsertLineItem", state.ID, item).Get(ctx, nil); err != nil {
				return state, err
			}

			return *domain.BillWithTotals(&state), nil
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

	if err := workflow.SetUpdateHandler(ctx, UpdateCloseBill,
		func(ctx workflow.Context, _ struct{}) (domain.Bill, error) {
			if state.Status == domain.BillStatusClosed {
				return *domain.BillWithTotals(&state), nil
			}

			now := workflow.Now(ctx).UTC()
			state.Status = domain.BillStatusClosed
			state.ClosedAt = &now

			if err := waitForBill(ctx); err != nil {
				return state, err
			}

			if err := workflow.ExecuteActivity(actCtx, "MarkBillClosed", state.ID, now).Get(ctx, nil); err != nil {
				return state, err
			}

			return *domain.BillWithTotals(&state), nil
		},
	); err != nil {
		return state, err
	}

	if err := workflow.ExecuteActivity(actCtx, "InsertBill", state).Get(ctx, nil); err != nil {
		return state, err
	}

	billSaved = true

	delay := state.PeriodEnd.Sub(workflow.Now(ctx))
	if delay < 0 {
		delay = 0
	}

	if _, err := workflow.AwaitWithTimeout(ctx, delay, func() bool {
		return state.Status == domain.BillStatusClosed
	}); err != nil {
		return state, err
	}

	if state.Status == domain.BillStatusOpen {
		now := workflow.Now(ctx).UTC()
		state.Status = domain.BillStatusClosed
		state.ClosedAt = &now
		if err := workflow.ExecuteActivity(actCtx, "MarkBillClosed", state.ID, now).Get(ctx, nil); err != nil {
			return state, err
		}
	}

	if err := workflow.Await(ctx, func() bool { return workflow.AllHandlersFinished(ctx) }); err != nil {
		return state, err
	}

	return *domain.BillWithTotals(&state), nil
}

// WorkflowID returns the Temporal workflow id for a bill.
func WorkflowID(billID string) string {
	return "bill/" + billID
}
