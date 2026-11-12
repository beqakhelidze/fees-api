package workflows

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"encore.app/fees/domain"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

func registerBillActivities(env *testsuite.TestWorkflowEnvironment) {
	env.RegisterActivityWithOptions(
		func(context.Context, domain.Bill) error { return nil },
		activity.RegisterOptions{Name: "InsertBill"},
	)
	env.RegisterActivityWithOptions(
		func(context.Context, string, domain.LineItem) error { return nil },
		activity.RegisterOptions{Name: "InsertLineItem"},
	)
	env.RegisterActivityWithOptions(
		func(context.Context, string, time.Time) error { return nil },
		activity.RegisterOptions{Name: "MarkBillClosed"},
	)
}

func TestBillWorkflow_AddLineItemAndClose(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	registerBillActivities(env)

	env.OnActivity("InsertBill", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("InsertLineItem", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("MarkBillClosed", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	now := time.Now().UTC()
	input := BillInput{
		ID:          "bill-1",
		PeriodStart: now,
		PeriodEnd:   now.Add(time.Hour),
	}
	item := domain.LineItem{
		ID:          "li-1",
		Description: "ATM fee",
		AmountMinor: 200,
		Currency:    domain.CurrencyGEL,
	}

	var addBill domain.Bill
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(UpdateAddLineItem, "add-1", &testsuite.TestUpdateCallback{
			OnReject: func(err error) {
				require.Fail(t, "add should not be rejected", err)
			},
			OnAccept: func() {},
			OnComplete: func(result any, err error) {
				require.NoError(t, err)
				bill, ok := result.(domain.Bill)
				require.True(t, ok)
				addBill = bill
			},
		}, item)
	}, time.Millisecond)

	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(UpdateCloseBill, "close-1", &testsuite.TestUpdateCallback{
			OnReject: func(err error) {
				require.Fail(t, "close should not be rejected", err)
			},
			OnAccept:   func() {},
			OnComplete: func(any, error) {},
		}, struct{}{})
	}, 2*time.Millisecond)

	env.ExecuteWorkflow(BillWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result domain.Bill
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, domain.BillStatusClosed, result.Status)
	require.Len(t, result.LineItems, 1)
	require.Equal(t, "li-1", result.LineItems[0].ID)
	require.Equal(t, []domain.Money{{AmountMinor: 200, Currency: domain.CurrencyGEL}}, result.Totals)
	require.NotNil(t, result.ClosedAt)

	require.Equal(t, domain.BillStatusOpen, addBill.Status)
	require.Len(t, addBill.LineItems, 1)
	env.AssertExpectations(t)
}

func TestBillWorkflow_RejectAddWhenClosed(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	registerBillActivities(env)

	env.OnActivity("InsertBill", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("MarkBillClosed", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	now := time.Now().UTC()
	input := BillInput{
		ID:          "bill-2",
		PeriodStart: now,
		PeriodEnd:   now.Add(time.Hour),
	}
	item := domain.LineItem{
		ID:          "li-1",
		Description: "late fee",
		AmountMinor: 100,
		Currency:    domain.CurrencyUSD,
	}

	var rejectErr error
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(UpdateCloseBill, "close-1", &testsuite.TestUpdateCallback{
			OnReject: func(err error) {
				require.Fail(t, "close should not be rejected", err)
			},
			OnAccept:   func() {},
			OnComplete: func(any, error) {},
		}, struct{}{})

		// Same callback: workflow would exit soon after close, so a later timer may never fire.
		env.UpdateWorkflow(UpdateAddLineItem, "add-after-close", &testsuite.TestUpdateCallback{
			OnReject: func(err error) {
				rejectErr = err
			},
			OnAccept: func() {
				require.Fail(t, "add after close should be rejected")
			},
			OnComplete: func(any, error) {},
		}, item)
	}, time.Millisecond)

	env.ExecuteWorkflow(BillWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Error(t, rejectErr)
	require.Contains(t, rejectErr.Error(), "bill is closed")
}

func TestBillWorkflow_RejectDuplicateLineItem(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	registerBillActivities(env)

	env.OnActivity("InsertBill", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("InsertLineItem", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("MarkBillClosed", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	now := time.Now().UTC()
	input := BillInput{
		ID:          "bill-3",
		PeriodStart: now,
		PeriodEnd:   now.Add(time.Hour),
	}
	item := domain.LineItem{
		ID:          "li-1",
		Description: "ATM fee",
		AmountMinor: 200,
		Currency:    domain.CurrencyGEL,
	}

	var rejectErr error
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(UpdateAddLineItem, "add-1", &testsuite.TestUpdateCallback{
			OnReject: func(err error) {
				require.Fail(t, "first add should not be rejected", err)
			},
			OnAccept:   func() {},
			OnComplete: func(any, error) {},
		}, item)
	}, time.Millisecond)

	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(UpdateAddLineItem, "add-dup", &testsuite.TestUpdateCallback{
			OnReject: func(err error) {
				rejectErr = err
			},
			OnAccept: func() {
				require.Fail(t, "duplicate add should be rejected")
			},
			OnComplete: func(any, error) {},
		}, item)
	}, 2*time.Millisecond)

	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(UpdateCloseBill, "close-1", &testsuite.TestUpdateCallback{
			OnReject:   func(error) {},
			OnAccept:   func() {},
			OnComplete: func(any, error) {},
		}, struct{}{})
	}, 3*time.Millisecond)

	env.ExecuteWorkflow(BillWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Error(t, rejectErr)
	require.True(t, strings.Contains(rejectErr.Error(), "already exists"))

	var result domain.Bill
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Len(t, result.LineItems, 1)
}

func TestBillWorkflow_AddLineItemPersistFailureRollsBack(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	registerBillActivities(env)

	env.OnActivity("InsertBill", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("InsertLineItem", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("db down"))
	env.OnActivity("MarkBillClosed", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	now := time.Now().UTC()
	input := BillInput{
		ID:          "bill-persist-fail",
		PeriodStart: now,
		PeriodEnd:   now.Add(time.Hour),
	}
	item := domain.LineItem{
		ID:          "li-fail",
		Description: "ATM fee",
		AmountMinor: 200,
		Currency:    domain.CurrencyGEL,
	}

	var addErr error
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(UpdateAddLineItem, "add-fail", &testsuite.TestUpdateCallback{
			OnReject: func(err error) {
				require.Fail(t, "add should not be rejected", err)
			},
			OnAccept: func() {},
			OnComplete: func(_ any, err error) {
				addErr = err
			},
		}, item)
	}, time.Millisecond)

	// After InsertLineItem exhausts retries (default backoff), close the bill.
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(UpdateCloseBill, "close-1", &testsuite.TestUpdateCallback{
			OnReject:   func(error) {},
			OnAccept:   func() {},
			OnComplete: func(any, error) {},
		}, struct{}{})
	}, time.Hour)

	env.ExecuteWorkflow(BillWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	require.Error(t, addErr)
	var appErr *temporal.ApplicationError
	require.ErrorAs(t, addErr, &appErr)
	require.Equal(t, "PersistLineItem", appErr.Type())
	require.Contains(t, appErr.Message(), `failed to persist line item "li-fail"`)

	var result domain.Bill
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Empty(t, result.LineItems)
}

func TestBillWorkflow_AutoCloseWhenPeriodEnded(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	registerBillActivities(env)

	env.OnActivity("InsertBill", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("MarkBillClosed", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	now := time.Now().UTC()
	input := BillInput{
		ID:          "bill-4",
		PeriodStart: now.Add(-2 * time.Hour),
		PeriodEnd:   now.Add(-time.Hour),
	}

	env.ExecuteWorkflow(BillWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result domain.Bill
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, domain.BillStatusClosed, result.Status)
	require.NotNil(t, result.ClosedAt)
	require.Empty(t, result.LineItems)
	env.AssertExpectations(t)
}
