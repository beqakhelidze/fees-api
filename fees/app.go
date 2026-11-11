package fees

import (
	"context"
	"fmt"

	"encore.app/fees/activities"
	"encore.app/fees/repository"
	"encore.app/fees/service"
	"encore.app/fees/workflow"
	"encore.dev"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// Use an environment-specific task queue so one Temporal cluster can serve
// multiple Encore environments safely.
var (
	envName       = encore.Meta().Environment.Name
	feesTaskQueue = envName + "-fees"
)

//encore:service
type App struct {
	client client.Client
	worker worker.Worker
	bills  *service.BillService
}

func initApp() (*App, error) {
	c, err := client.Dial(client.Options{
		HostPort: cfg.TemporalServer,
	})
	if err != nil {
		return nil, fmt.Errorf("create temporal client: %w", err)
	}

	billRepo := repository.NewBillRepository(db)
	billService := service.NewBillService(c, feesTaskQueue, billRepo)
	acts := activities.New(billRepo)

	w := worker.New(c, feesTaskQueue, worker.Options{})
	w.RegisterWorkflow(workflow.BillWorkflow)
	w.RegisterActivity(acts.PersistBill)

	if err := w.Start(); err != nil {
		c.Close()
		return nil, fmt.Errorf("start temporal worker: %w", err)
	}

	return &App{
		client: c,
		worker: w,
		bills:  billService,
	}, nil
}

func (s *App) Shutdown(_ context.Context) {
	s.worker.Stop()
	s.client.Close()
}
