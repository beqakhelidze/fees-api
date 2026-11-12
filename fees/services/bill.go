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

func (s *BillService) Get(ctx context.Context, billID string) (*domain.Bill, error) {
	if strings.TrimSpace(billID) == "" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("bill id is required").Err()
	}

	stored, err := s.billRepo.GetByID(ctx, billID)

	if err != nil {
		if errors.Is(err, sqldb.ErrNoRows) {
			return nil, errs.B().Code(errs.NotFound).Msg("bill not found").Err()
		}
		return nil, err
	}

	return domain.BillWithTotals(stored), nil
}

func (s *BillService) Create(ctx context.Context, periodStart, periodEnd time.Time) (*domain.Bill, error) {
	if periodStart.IsZero() || periodEnd.IsZero() {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("period_start and period_end are required").Err()
	}
	if !periodEnd.After(periodStart) {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("period_end must be after period_start").Err()
	}
	if !periodEnd.UTC().After(time.Now().UTC()) {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("period_end must be in the future").Err()
	}

	billID := uuid.NewString()
	input := workflows.BillInput{
		ID:          billID,
		PeriodStart: periodStart.UTC(),
		PeriodEnd:   periodEnd.UTC(),
	}

	startOp := s.temporal.NewWithStartWorkflowOperation(
		client.StartWorkflowOptions{
			ID:                       workflows.WorkflowID(billID),
			TaskQueue:                s.taskQueue,
			WorkflowIDConflictPolicy: enumspb.WORKFLOW_ID_CONFLICT_POLICY_FAIL,
		},
		workflows.BillWorkflow,
		input,
	)

	update, err := s.temporal.UpdateWithStartWorkflow(ctx, client.UpdateWithStartWorkflowOptions{
		StartWorkflowOperation: startOp,
		UpdateOptions: client.UpdateWorkflowOptions{
			UpdateName:   workflows.UpdateWaitReady,
			Args:         []interface{}{struct{}{}},
			WaitForStage: client.WorkflowUpdateStageCompleted,
		},
	})
	if err != nil {
		return nil, temporalError(err)
	}

	var bill domain.Bill
	if err := update.Get(ctx, &bill); err != nil {
		return nil, temporalError(err)
	}
	return domain.BillWithTotals(&bill), nil
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

	item.Currency = domain.NormalizeCurrency(item.Currency)

	update, err := s.temporal.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		WorkflowID:   workflows.WorkflowID(billID),
		UpdateName:   workflows.UpdateAddLineItem,
		Args:         []interface{}{item},
		WaitForStage: client.WorkflowUpdateStageCompleted,
	})

	if err != nil {
		if bill, dbErr := s.billRepo.GetByID(ctx, billID); dbErr == nil && bill.Status == domain.BillStatusClosed {
			return nil, errs.B().Code(errs.FailedPrecondition).Msg("bill is closed").Err()
		}
		return nil, temporalError(err)
	}

	var bill domain.Bill
	if err := update.Get(ctx, &bill); err != nil {
		return nil, temporalError(err)
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
		if bill, dbErr := s.billRepo.GetByID(ctx, billID); dbErr == nil && bill.Status == domain.BillStatusClosed {
			return domain.BillWithTotals(bill), nil
		}
		return nil, temporalError(err)
	}

	var bill domain.Bill
	if err := update.Get(ctx, &bill); err != nil {
		return nil, temporalError(err)
	}

	return domain.BillWithTotals(&bill), nil
}

func temporalError(err error) error {
	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		return errs.B().Code(errs.NotFound).Msg("bill not found").Err()
	}

	var appErr *temporal.ApplicationError
	if errors.As(err, &appErr) {
		code := errs.FailedPrecondition
		if appErr.Type() == "PersistLineItem" {
			code = errs.Unavailable
		}
		return errs.B().Code(code).Msg(appErr.Message()).Err()
	}

	return err
}
