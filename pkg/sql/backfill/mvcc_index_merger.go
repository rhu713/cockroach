// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package backfill

import (
	"context"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"time"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/tabledesc"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfra"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/ctxgroup"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/mon"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/logtags"
)

// indexBackfillMergeBatchSize is the maximum number of rows we attempt to merge
// in a single transaction during the merging process.
var indexBackfillMergeBatchSize = settings.RegisterIntSetting(
	settings.TenantWritable,
	"bulkio.index_backfill.merge_batch_size",
	"the number of rows we merge between temporary and adding indexes in a single batch",
	1000,
	settings.NonNegativeInt, /* validateFn */
)

// indexBackfillMergeBatchBytes is the maximum number of bytes we attempt to
// merge from the temporary index in a single transaction during the merging
// process.
var indexBackfillMergeBatchBytes = settings.RegisterIntSetting(
	settings.TenantWritable,
	"bulkio.index_backfill.merge_batch_bytes",
	"the max number of bytes we merge between temporary and adding indexes in a single batch",
	16<<20,
	settings.NonNegativeInt,
)

// IndexBackfillMerger is a processor that merges entries from the corresponding
// temporary index to a new index.
type IndexBackfillMerger struct {
	spec execinfrapb.IndexBackfillMergerSpec

	desc catalog.TableDescriptor

	out execinfra.ProcOutputHelper

	flowCtx *execinfra.FlowCtx

	evalCtx *tree.EvalContext

	output execinfra.RowReceiver

	mon          *mon.BytesMonitor
	boundAccount mon.BoundAccount
}

// OutputTypes is always nil.
func (ibm *IndexBackfillMerger) OutputTypes() []*types.T {
	return nil
}

// MustBeStreaming is always false.
func (ibm *IndexBackfillMerger) MustBeStreaming() bool {
	return false
}

const indexBackfillMergeProgressReportInterval = 10 * time.Second

// Run runs the processor.
func (ibm *IndexBackfillMerger) Run(ctx context.Context) {
	log.Info(ctx, "Run called")
	opName := "IndexBackfillMerger"
	ctx = logtags.AddTag(ctx, opName, int(ibm.spec.Table.ID))
	ctx, span := execinfra.ProcessorSpan(ctx, opName)
	defer span.Finish()
	defer ibm.output.ProducerDone()
	defer execinfra.SendTraceData(ctx, ibm.output)

	// TODO: move this into spec
	readAsOf := ibm.flowCtx.Cfg.DB.Clock().Now()

	mu := struct {
		syncutil.Mutex
		completedSpans   []roachpb.Span
		completedSpanIdx []int32
	}{}

	progCh := make(chan execinfrapb.RemoteProducerMetadata_BulkProcessorProgress)
	pushProgress := func() {
		mu.Lock()
		var prog execinfrapb.RemoteProducerMetadata_BulkProcessorProgress
		prog.CompletedSpans = append(prog.CompletedSpans, mu.completedSpans...)
		mu.completedSpans = nil
		prog.CompletedSpanIdx = append(prog.CompletedSpanIdx, mu.completedSpanIdx...)
		mu.completedSpanIdx = nil
		mu.Unlock()

		progCh <- prog
	}

	semaCtx := tree.MakeSemaContext()
	if err := ibm.out.Init(&execinfrapb.PostProcessSpec{}, nil, &semaCtx, ibm.flowCtx.NewEvalCtx()); err != nil {
		ibm.output.Push(nil, &execinfrapb.ProducerMetadata{Err: err})
		return
	}
	log.Info(ctx, "after init called")

	// stopProgress will be closed when there is no more progress to report.
	stopProgress := make(chan struct{})
	g := ctxgroup.WithContext(ctx)
	g.GoCtx(func(ctx context.Context) error {
		tick := time.NewTicker(indexBackfillMergeProgressReportInterval)
		defer tick.Stop()
		done := ctx.Done()
		for {
			select {
			case <-done:
				return ctx.Err()
			case <-stopProgress:
				return nil
			case <-tick.C:
				pushProgress()
			}
		}
	})

	mergeCh := make(chan MergeChunk, 10)

	g.GoCtx(func(ctx context.Context) error {
		defer close(mergeCh)
		log.Info(ctx, "before spans")
		for i := range ibm.spec.Spans {
			sp := ibm.spec.Spans[i]
			idx := ibm.spec.SpanIdx[i]
			log.Infof(ctx, "merging span %v", sp)

			key := sp.Key
			for key != nil {
				nextKey, err := ibm.Scan(ctx, idx, ibm.spec.AddedIndexes[idx], key, sp.EndKey, readAsOf, mergeCh)
				if err != nil {
					return err
				}
				key = nextKey
			}
		}
		return nil
	})

	numWorkers := 5

	g.GoCtx(func(ctx context.Context) error {
		defer close(stopProgress)
		// TODO(rui): some room for improvement on single threaded
		// implementation, e.g. run merge for spec spans in parallel.

		for worker := 0; worker < numWorkers; worker++ {
			g.GoCtx(func(ctx context.Context) error {
				for mergeChunk := range mergeCh {
					err := ibm.Merge(ctx, ibm.evalCtx.Codec, ibm.desc, ibm.spec.TemporaryIndexes[mergeChunk.spanIdx],
						ibm.spec.AddedIndexes[mergeChunk.spanIdx], mergeChunk.keys, mergeChunk.completedSpan)
					if err != nil {
						return err
					}

					log.Infof(ctx, "before lock for span %v", mergeChunk.completedSpan)
					mu.Lock()
					log.Infof(ctx, "after lock for span %v", mergeChunk.completedSpan)
					mu.completedSpans = append(mu.completedSpans, mergeChunk.completedSpan)
					mu.completedSpanIdx = append(mu.completedSpanIdx, mergeChunk.spanIdx)
					mu.Unlock()

					if knobs, ok := ibm.flowCtx.Cfg.TestingKnobs.IndexBackfillMergerTestingKnobs.(*IndexBackfillMergerTestingKnobs); ok {
						if knobs != nil && knobs.PushesProgressEveryChunk {
							pushProgress()
						}
					}

				}
				return nil
			})
		}

		return nil
	})

	var err error
	go func() {
		defer close(progCh)
		err = g.Wait()
	}()

	for prog := range progCh {
		p := prog
		if p.CompletedSpans != nil {
			log.VEventf(ctx, 2, "sending coordinator completed spans: %+v", p.CompletedSpans)
		}
		ibm.output.Push(nil, &execinfrapb.ProducerMetadata{BulkProcessorProgress: &p})
	}

	if err != nil {
		ibm.output.Push(nil, &execinfrapb.ProducerMetadata{Err: err})
	}
}

var _ execinfra.Processor = &IndexBackfillMerger{}

type MergeChunk struct {
	completedSpan roachpb.Span
	keys          []roachpb.Key
	spanIdx       int32
}

func (ibm *IndexBackfillMerger) Scan(
	ctx context.Context,
	spanIdx int32,
	destinationID descpb.IndexID,
	startKey roachpb.Key,
	endKey roachpb.Key,
	readAsOf hlc.Timestamp,
	mergeCh chan MergeChunk,
) (roachpb.Key, error) {
	if knobs, ok := ibm.flowCtx.Cfg.TestingKnobs.IndexBackfillMergerTestingKnobs.(*IndexBackfillMergerTestingKnobs); ok {
		if knobs != nil && knobs.RunBeforeMergeChunk != nil {
			if err := knobs.RunBeforeMergeChunk(startKey); err != nil {
				return nil, err
			}
		}
	}
	chunkSize := indexBackfillMergeBatchSize.Get(&ibm.evalCtx.Settings.SV)
	chunkBytes := indexBackfillMergeBatchBytes.Get(&ibm.evalCtx.Settings.SV)

	var nextStart roachpb.Key
	var br *roachpb.BatchResponse
	if err := ibm.flowCtx.Cfg.DB.Txn(ctx, func(ctx context.Context, txn *kv.Txn) error {
		if err := txn.SetFixedTimestamp(ctx, readAsOf); err != nil {
			return err
		}
		// For now just grab all of the destination KVs and merge the corresponding entries.
		log.Infof(ctx, "merging batch [%s, %s) into index %d", startKey, endKey, destinationID)
		var ba roachpb.BatchRequest
		ba.TargetBytes = chunkBytes
		//if err := ibm.boundAccount.Grow(ctx, chunkBytes); err != nil {
		//	return errors.Errorf("failed to fetch keys to merge from temp index")
		//}
		//defer ibm.boundAccount.Shrink(ctx, chunkBytes)

		ba.MaxSpanRequestKeys = chunkSize
		ba.Add(&roachpb.ScanRequest{
			RequestHeader: roachpb.RequestHeader{
				Key:    startKey,
				EndKey: endKey,
			},
			ScanFormat: roachpb.KEY_VALUES,
		})
		var pErr *roachpb.Error
		br, pErr = txn.Send(ctx, ba)
		if pErr != nil {
			return pErr.GoError()
		}
		return nil
	}); err != nil {
		return nil, err
	}

	resp := br.Responses[0].GetScan()
	chunk := MergeChunk{
		spanIdx: spanIdx,
	}
	if len(resp.Rows) == 0 {
		chunk.completedSpan = roachpb.Span{Key: startKey, EndKey: endKey}
	} else {
		nextStart = resp.Rows[len(resp.Rows)-1].Key.Next()
		chunk.completedSpan = roachpb.Span{Key: startKey, EndKey: nextStart}

		for i := range resp.Rows {
			chunk.keys = append(chunk.keys, resp.Rows[i].Key)
		}
	}
	mergeCh <- chunk
	return nextStart, nil
}

// Merge merges the entries from startKey to endKey from the index with sourceID
// into the index with destinationID, up to a maximum of chunkSize entries.
func (ibm *IndexBackfillMerger) Merge(
	ctx context.Context,
	codec keys.SQLCodec,
	table catalog.TableDescriptor,
	sourceID descpb.IndexID,
	destinationID descpb.IndexID,
	sourceKeys []roachpb.Key,
	sourceSpan roachpb.Span,
) error {
	sourcePrefix := rowenc.MakeIndexKeyPrefix(codec, table.GetID(), sourceID)
	prefixLen := len(sourcePrefix)
	destPrefix := rowenc.MakeIndexKeyPrefix(codec, table.GetID(), destinationID)

	destKey := make([]byte, len(destPrefix))

	err := ibm.flowCtx.Cfg.DB.Txn(ctx, func(ctx context.Context, txn *kv.Txn) error {
		var deletedCount int
		txn.AddCommitTrigger(func(ctx context.Context) {
			log.VInfof(ctx, 2, "merged batch of %d keys (%d deletes) (span: %s) (commit timestamp: %s)",
				len(sourceKeys),
				deletedCount,
				sourceSpan,
				txn.CommitTimestamp(),
			)
		})
		if len(sourceKeys) == 0 {
			return nil
		}

		rb := txn.NewBatch()
		for i := range sourceKeys {
			rb.Get(sourceKeys[i])
		}
		if err := txn.Run(ctx, rb); err != nil {
			return err
		}

		wb := txn.NewBatch()
		var memUsedInMerge int64
		for i := range rb.Results {
			sourceKV := &rb.Results[i].Rows[0]
			if len(sourceKV.Key) < prefixLen {
				return errors.Errorf("key for index entry %v does not start with prefix %v", sourceKV, sourcePrefix)
			}

			destKey = destKey[:0]
			destKey = append(destKey, destPrefix...)
			destKey = append(destKey, sourceKV.Key[prefixLen:]...)

			mergedEntry, deleted, err := mergeEntry(sourceKV, destKey)
			if err != nil {
				return err
			}

			if deleted {
				deletedCount++
				wb.Del(mergedEntry.Key)
				//if err := ibm.boundAccount.Grow(ctx, int64(len(mergedEntry.Key))); err != nil {
				//	return errors.Errorf("failed to allocate space to merge delete from temp index")
				//}
				memUsedInMerge += int64(len(mergedEntry.Key))
			} else {
				wb.Put(mergedEntry.Key, mergedEntry.Value)
				//if err := ibm.boundAccount.Grow(ctx, int64(len(mergedEntry.Key)+len(mergedEntry.Value.RawBytes))); err != nil {
				//	return errors.Errorf("failed to allocate space to merge put from temp index")
				//}
				memUsedInMerge += int64(len(mergedEntry.Key) + len(mergedEntry.Value.RawBytes))
			}
		}
		//defer ibm.boundAccount.Shrink(ctx, memUsedInMerge)
		if err := txn.Run(ctx, wb); err != nil {
			return err
		}

		if knobs, ok := ibm.flowCtx.Cfg.TestingKnobs.IndexBackfillMergerTestingKnobs.(*IndexBackfillMergerTestingKnobs); ok {
			if knobs != nil && knobs.RunDuringMergeTxn != nil {
				if err := knobs.RunDuringMergeTxn(ctx, txn, sourceSpan.Key, sourceSpan.EndKey); err != nil {
					return err
				}
			}
		}
		return nil
	})

	return err
}

func mergeEntry(sourceKV *kv.KeyValue, destKey roachpb.Key) (*kv.KeyValue, bool, error) {
	var destTagAndData []byte
	var deleted bool

	tempWrapper, err := rowenc.DecodeWrapper(sourceKV.Value)
	if err != nil {
		return nil, false, err
	}

	if tempWrapper.Deleted {
		deleted = true
	} else {
		destTagAndData = tempWrapper.Value
	}

	value := &roachpb.Value{}
	value.SetTagAndData(destTagAndData)

	return &kv.KeyValue{
		Key:   destKey.Clone(),
		Value: value,
	}, deleted, nil
}

// NewIndexBackfillMerger creates a new IndexBackfillMerger.
func NewIndexBackfillMerger(
	ctx context.Context,
	flowCtx *execinfra.FlowCtx,
	spec execinfrapb.IndexBackfillMergerSpec,
	output execinfra.RowReceiver,
) (*IndexBackfillMerger, error) {
	mergerMon := execinfra.NewMonitor(ctx, flowCtx.Cfg.BackfillerMonitor,
		"index-backfiller-merger-mon")

	ibm := &IndexBackfillMerger{
		spec:    spec,
		desc:    tabledesc.NewUnsafeImmutable(&spec.Table),
		flowCtx: flowCtx,
		evalCtx: flowCtx.NewEvalCtx(),
		output:  output,
		mon:     mergerMon,
	}

	ibm.boundAccount = mergerMon.MakeBoundAccount()
	return ibm, nil
}

// IndexBackfillMergerTestingKnobs is for testing the distributed processors for
// the index backfill merge step.
type IndexBackfillMergerTestingKnobs struct {
	// RunBeforeMergeChunk is called once before the merge of each chunk. It is
	// called with starting key of the chunk.
	RunBeforeMergeChunk func(startKey roachpb.Key) error

	RunDuringMergeTxn func(ctx context.Context, txn *kv.Txn, startKey roachpb.Key, endKey roachpb.Key) error

	// PushesProgressEveryChunk forces the process to push the merge process after
	// every chunk.
	PushesProgressEveryChunk bool
}

var _ base.ModuleTestingKnobs = &IndexBackfillMergerTestingKnobs{}

// ModuleTestingKnobs implements the base.ModuleTestingKnobs interface.
func (*IndexBackfillMergerTestingKnobs) ModuleTestingKnobs() {}
