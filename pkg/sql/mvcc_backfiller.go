// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package sql

import (
	"context"
	"github.com/cockroachdb/cockroach/pkg/jobs"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobspb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/errors"
	"golang.org/x/sync/errgroup"
	"time"

	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/backfill"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descs"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlutil"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
)

type IndexBackfillerMergePlanner struct {
	execCfg   *ExecutorConfig
	ieFactory sqlutil.SessionBoundInternalExecutorFactory
}

func NewIndexBackfillerMergePlanner(
	execCfg *ExecutorConfig, ieFactory sqlutil.SessionBoundInternalExecutorFactory,
) *IndexBackfillerMergePlanner {
	return &IndexBackfillerMergePlanner{execCfg: execCfg, ieFactory: ieFactory}
}

func (im *IndexBackfillerMergePlanner) plan(
	ctx context.Context,
	tableDesc catalog.TableDescriptor,
	readTimestamp hlc.Timestamp,
	todoSpanList [][]roachpb.Span,
	addedIndexes, temporaryIndexes []descpb.IndexID,
	metaFn func(_ context.Context, meta *execinfrapb.ProducerMetadata) error,
) (func(context.Context) error, error) {
	var p *PhysicalPlan
	var evalCtx extendedEvalContext
	var planCtx *PlanningCtx

	// The txn is used to fetch a tableDesc, partition the spans and set the
	// evalCtx ts all of which is during planning of the DistSQL flow.
	if err := DescsTxn(ctx, im.execCfg, func(
		ctx context.Context, txn *kv.Txn, descriptors *descs.Collection,
	) error {
		evalCtx = createSchemaChangeEvalCtx(ctx, im.execCfg, txn.ReadTimestamp(), descriptors)
		planCtx = im.execCfg.DistSQLPlanner.NewPlanningCtx(ctx, &evalCtx, nil /* planner */, txn,
			true /* distribute */)
		chunkSize := indexBackfillBatchSize.Get(&im.execCfg.Settings.SV)

		spec, err := initIndexBackfillMergerSpec(*tableDesc.TableDesc(), chunkSize, readTimestamp, addedIndexes, temporaryIndexes)
		if err != nil {
			return err
		}
		p, err = im.execCfg.DistSQLPlanner.createIndexBackfillerMergePhysicalPlan(planCtx, spec, todoSpanList)
		return err
	}); err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		cbw := MetadataCallbackWriter{rowResultWriter: &errOnlyResultWriter{}, fn: metaFn}
		recv := MakeDistSQLReceiver(
			ctx,
			&cbw,
			tree.Rows, /* stmtType - doesn't matter here since no result are produced */
			im.execCfg.RangeDescriptorCache,
			nil, /* txn - the flow does not run wholly in a txn */
			im.execCfg.Clock,
			evalCtx.Tracing,
			im.execCfg.ContentionRegistry,
			nil, /* testingPushCallback */
		)
		defer recv.Release()
		evalCtxCopy := evalCtx
		im.execCfg.DistSQLPlanner.Run(
			planCtx,
			nil, /* txn - the processors manage their own transactions */
			p, recv, &evalCtxCopy,
			nil, /* finishedSetupFn */
		)()
		return cbw.Err()
	}, nil
}

type MergeProgress struct {
	todoSpans                      [][]roachpb.Span
	mutationIdx                    []int
	readTimestamp                  hlc.Timestamp
	addedIndexes, temporaryIndexes []descpb.IndexID
}

type IndexMergeTracker struct {
	mu struct {
		syncutil.Mutex
		progress *MergeProgress
	}
}

func NewIndexMergeTracker(progress *MergeProgress) *IndexMergeTracker {
	imt := IndexMergeTracker{}
	imt.mu.progress = progress
	return &imt
}

func (imt *IndexMergeTracker) FlushCheckpoint(ctx context.Context, job *jobs.Job) error {
	updatedTodoSpans := imt.GetProgress()

	if updatedTodoSpans.todoSpans == nil {
		return nil
	}

	details, ok := job.Details().(jobspb.SchemaChangeDetails)
	if !ok {
		return errors.Errorf("expected SchemaChangeDetails job type, got %T", job.Details())
	}

	for idx := range updatedTodoSpans.todoSpans {
		details.ResumeSpanList[updatedTodoSpans.mutationIdx[idx]].ResumeSpans = updatedTodoSpans.todoSpans[idx]
	}

	return job.SetDetails(ctx, nil, details)
}

func (imt *IndexMergeTracker) FlushFractionCompleted(ctx context.Context) error {
	// TODO(rui): The backfiller currently doesn't have a good way to report the
	// total progress of mutations that occur in multiple stages that
	// independently report progress. So fraction tracking of the merge will be
	// unimplemented for now and the progress fraction will report only the
	// progress of the backfilling stage.
	return nil
}

func (imt *IndexMergeTracker) UpdateMergeProgress(ctx context.Context, progress *MergeProgress) {
	imt.mu.Lock()
	imt.mu.progress = progress
	imt.mu.Unlock()
}

func (imt *IndexMergeTracker) GetProgress() *MergeProgress {
	imt.mu.Lock()
	defer imt.mu.Unlock()
	return imt.mu.progress
}

type PeriodicMergeProgressFlusher struct {
	clock                                timeutil.TimeSource
	checkpointInterval, fractionInterval func() time.Duration
}

func (p *PeriodicMergeProgressFlusher) StartPeriodicUpdates(
	ctx context.Context, tracker *IndexMergeTracker, job *jobs.Job,
) (stop func() error) {
	stopCh := make(chan struct{})
	runPeriodicWrite := func(
		ctx context.Context,
		write func(context.Context) error,
		interval func() time.Duration,
	) error {
		timer := p.clock.NewTimer()
		defer timer.Stop()
		for {
			timer.Reset(interval())
			select {
			case <-stopCh:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.Ch():
				timer.MarkRead()
				if err := write(ctx); err != nil {
					return err
				}
			}
		}
	}
	var g errgroup.Group
	g.Go(func() error {
		return runPeriodicWrite(
			ctx, tracker.FlushFractionCompleted, p.fractionInterval)
	})
	g.Go(func() error {
		return runPeriodicWrite(
			ctx,
			func(ctx context.Context) error {
				return tracker.FlushCheckpoint(ctx, job)
			},
			p.checkpointInterval)
	})
	toClose := stopCh // make the returned function idempotent
	return func() error {
		if toClose != nil {
			close(toClose)
			toClose = nil
		}
		return g.Wait()
	}

}

func newPeriodicProgressFlusher(settings *cluster.Settings) PeriodicMergeProgressFlusher {
	clock := timeutil.DefaultTimeSource{}
	getCheckpointInterval := func() time.Duration {
		return backfill.IndexBackfillCheckpointInterval.Get(&settings.SV)
	}
	// fractionInterval is copied from the logic in existing backfill code.
	// TODO(ajwerner): Add a cluster setting to control this.
	const fractionInterval = 10 * time.Second
	getFractionInterval := func() time.Duration { return fractionInterval }
	return PeriodicMergeProgressFlusher{
		clock:              clock,
		checkpointInterval: getCheckpointInterval,
		fractionInterval:   getFractionInterval,
	}
}
