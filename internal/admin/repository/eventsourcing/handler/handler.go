package handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/zitadel/logging"

	"github.com/zitadel/zitadel/internal/admin/repository/eventsourcing/view"
	"github.com/zitadel/zitadel/internal/api/authz"
	"github.com/zitadel/zitadel/internal/database"
	"github.com/zitadel/zitadel/internal/eventstore"
	"github.com/zitadel/zitadel/internal/eventstore/handler/v2"
	handler2 "github.com/zitadel/zitadel/internal/eventstore/handler/v2"
	"github.com/zitadel/zitadel/internal/static"
)

type Config struct {
	Client     *database.DB
	Eventstore *eventstore.Eventstore

	BulkLimit             uint64
	FailureCountUntilSkip uint64
	TransactionDuration   time.Duration
	Handlers              map[string]*ConfigOverwrites
	ActiveInstancer       interface {
		ActiveInstances() []string
	}
}

type ConfigOverwrites struct {
	MinimumCycleDuration time.Duration
}

var projections []*handler.Handler

func Register(ctx context.Context, config Config, view *view.View, static static.Storage) {
	if static == nil {
		return
	}

	// make sure the slice does not contain old values
	projections = nil

	projections = append(projections, newStyling(ctx,
		config.overwrite("Styling"),
		static,
		view,
	))
}

func Projections() []*handler2.Handler {
	return projections
}

func Start(ctx context.Context) {
	for _, projection := range projections {
		projection.Start(ctx)
	}
}

func ProjectInstance(ctx context.Context) error {
	for i, projection := range projections {
		logging.WithFields("name", projection.ProjectionName(), "instance", authz.GetInstance(ctx).InstanceID(), "index", fmt.Sprintf("%d/%d", i, len(projections))).Info("starting admin projection")
		for {
			_, err := projection.Trigger(ctx)
			if err == nil {
				break
			}
			var pgErr *pgconn.PgError
			errors.As(err, &pgErr)
			if pgErr.Code != database.PgUniqueConstraintErrorCode {
				return err
			}
			logging.WithFields("name", projection.ProjectionName(), "instance", authz.GetInstance(ctx).InstanceID()).WithError(err).Debug("admin projection failed because of unique constraint, retrying")
		}
		logging.WithFields("name", projection.ProjectionName(), "instance", authz.GetInstance(ctx).InstanceID(), "index", fmt.Sprintf("%d/%d", i, len(projections))).Info("admin projection done")
	}
	return nil
}

func (config Config) overwrite(viewModel string) handler2.Config {
	c := handler2.Config{
		Client:              config.Client,
		Eventstore:          config.Eventstore,
		BulkLimit:           uint16(config.BulkLimit),
		RequeueEvery:        3 * time.Minute,
		MaxFailureCount:     uint8(config.FailureCountUntilSkip),
		TransactionDuration: config.TransactionDuration,
		ActiveInstancer:     config.ActiveInstancer,
	}
	overwrite, ok := config.Handlers[viewModel]
	if !ok {
		return c
	}
	if overwrite.MinimumCycleDuration > 0 {
		c.RequeueEvery = overwrite.MinimumCycleDuration
	}
	return c
}
