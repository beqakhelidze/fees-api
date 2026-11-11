package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"encore.app/fees/domain"
	"encore.app/fees/repositories"
	"encore.app/fees/workflows"
	"encore.dev/beta/errs"
	"encore.dev/storage/sqldb"
	"github.com/google/uuid"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

func NewBillService(temporal client.Client, taskQueue string, billRepo repositories.BillRepository) *BillService {
	return &BillService{
		temporal:  temporal,
		taskQueue: taskQueue,
		billRepo:  billRepo,
	}
}

type BillService struct {
	temporal  client.Client
	taskQueue string
	billRepo  repositories.BillRepository
}

func (s *BillService) Create(ctx context.Context, periodStart, periodEnd time.Time) (*domain.Bill, error) {
	if periodStart.IsZero() || periodEnd.IsZero() {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("period_start and period_end are required").Err()
	}
	if !periodEnd.After(periodStart) {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("period_end must be after period_start").Err()
	}

	billID := uuid.NewString()
	input := workflows.BillInput{
		ID:          billID,
		PeriodStart: periodStart.UTC(),
		PeriodEnd:   periodEnd.UTC(),
	}

	_, err := s.temporal.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                    workflows.WorkflowID(billID),
		TaskQueue:             s.taskQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
	}, workflows.BillWorkflow, input)
	if err != nil {
		return nil, err
	}

	return domain.BillWithTotals(&domain.Bill{
		ID:          billID,
		Status:      domain.BillStatusOpen,
		PeriodStart: input.PeriodStart,
		PeriodEnd:   input.PeriodEnd,
		LineItems:   []domain.LineItem{},
	}), nil
}

func (s *BillService) AddLineItem(ctx context.Context, billID string, item domain.LineItem) (*domain.Bill, error) {
	if strings.TrimSpace(billID) == "" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("bill id is required").Err()
	}
	if strings.TrimSpace(item.ID) == "" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("line item id is required").Err()
	}
	if strings.TrimSpace(item.Description) == "" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("description is required").Err()
	}
	if err := domain.ValidateMoney(domain.Money{AmountMinor: item.AmountMinor, Currency: item.Currency}); err != nil {
		return nil, err
	}

	item.Currency = domain.NormalizeCurrency(item.Currency)

	update, err := s.temporal.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		WorkflowID:   workflows.WorkflowID(billID),
		UpdateName:   workflows.UpdateAddLineItem,
		Args:         []interface{}{item},
		WaitForStage: client.WorkflowUpdateStageCompleted,
	})
	if err != nil {
		return nil, s.updateError(ctx, billID, err)
	}

	var bill domain.Bill
	if err := update.Get(ctx, &bill); err != nil {
		return nil, s.updateError(ctx, billID, err)
	}
	return domain.BillWithTotals(&bill), nil
}

func (s *BillService) Close(ctx context.Context, billID string) (*domain.Bill, error) {
	if strings.TrimSpace(billID) == "" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("bill id is required").Err()
	}

	update, err := s.temporal.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		WorkflowID:   workflows.WorkflowID(billID),
		UpdateName:   workflows.UpdateCloseBill,
		Args:         []interface{}{struct{}{}},
		WaitForStage: client.WorkflowUpdateStageCompleted,
	})
	if err != nil {
		return nil, s.updateError(ctx, billID, err)
	}

	var bill domain.Bill
	if err := update.Get(ctx, &bill); err != nil {
		return nil, s.updateError(ctx, billID, err)
	}
	return domain.BillWithTotals(&bill), nil
}

func (s *BillService) Get(ctx context.Context, billID string) (*domain.Bill, error) {
	if strings.TrimSpace(billID) == "" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("bill id is required").Err()
	}

	var bill domain.Bill
	encoded, err := s.temporal.QueryWorkflow(ctx, workflows.WorkflowID(billID), "", workflows.QueryGetBill)
	if err == nil {
		if err := encoded.Get(&bill); err != nil {
			return nil, err
		}
		return domain.BillWithTotals(&bill), nil
	}

	stored, dbErr := s.billRepo.GetByID(ctx, billID)
	
	if dbErr == nil {
		return domain.BillWithTotals(stored), nil
	}

	if errors.Is(dbErr, sqldb.ErrNoRows) {
		return nil, temporalError(err)
	}
	return nil, dbErr
}

func (s *BillService) rejectIfClosed(ctx context.Context, id string) error {
	bill, err := s.billRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sqldb.ErrNoRows) {
			return nil
		}
		return err
	}
	if bill.Status == domain.BillStatusClosed {
		return errs.B().Code(errs.FailedPrecondition).Msg("bill is closed").Err()
	}
	return nil
}

func temporalError(err error) error {
	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		return errs.B().Code(errs.NotFound).Msg("bill not found").Err()
	}
	var appErr *temporal.ApplicationError
	if errors.As(err, &appErr) {
		return errs.B().Code(errs.FailedPrecondition).Msg(appErr.Message()).Err()
	}
	return err
}