package backfill

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/tabledesc"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfra"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/logtags"
	"github.com/pkg/errors"
)

type IndexBackfillMerger struct {
	spec execinfrapb.IndexBackfillMergerSpec

	desc catalog.TableDescriptor

	out execinfra.ProcOutputHelper

	flowCtx *execinfra.FlowCtx

	evalCtx *tree.EvalContext

	output execinfra.RowReceiver
}

func (ibm *IndexBackfillMerger) OutputTypes() []*types.T {
	// No output types.
	return nil
}

func (ibm *IndexBackfillMerger) MustBeStreaming() bool {
	return false
}

func (ibm *IndexBackfillMerger) Run(ctx context.Context) {
	opName := "indexMerger"
	ctx = logtags.AddTag(ctx, opName, int(ibm.spec.Table.ID))
	ctx, span := execinfra.ProcessorSpan(ctx, opName)
	defer span.Finish()
	defer ibm.output.ProducerDone()
	defer execinfra.SendTraceData(ctx, ibm.output)

	semaCtx := tree.MakeSemaContext()
	if err := ibm.out.Init(&execinfrapb.PostProcessSpec{}, nil, &semaCtx, ibm.flowCtx.NewEvalCtx()); err != nil {
		ibm.output.Push(nil, &execinfrapb.ProducerMetadata{Err: err})
		return
	}

	// TODO: improve single threaded naive implementation
	for i := range ibm.spec.Spans {
		sp := ibm.spec.Spans[i]
		idx := ibm.spec.SpanIdx[i]
		completedSpans, err := ibm.Merge(ctx, ibm.evalCtx.Codec, ibm.desc, ibm.spec.TemporaryIndexes[idx], ibm.spec.AddedIndexes[idx],
			[]roachpb.Span{sp}, ibm.spec.ReadTimestamp)

		var prog execinfrapb.RemoteProducerMetadata_BulkProcessorProgress
		for completed := range completedSpans {
			prog.CompletedSpans = append(prog.CompletedSpans, completedSpans[completed])
			prog.CompletedSpanIdx = append(prog.CompletedSpanIdx, idx)
		}

		ibm.output.Push(nil, &execinfrapb.ProducerMetadata{BulkProcessorProgress: &prog})

		if err != nil {
			ibm.output.Push(nil, &execinfrapb.ProducerMetadata{Err: err})
			return
		}
	}
}

var _ execinfra.Processor = &IndexBackfillMerger{}

// Merge merges the entries in the provide span sourceSpan from the index with
// sourceID into the index with destinationID. The function returns the spans
// from the source index that have finished merging, even in the presence of
// errors.
func (ibm *IndexBackfillMerger) Merge(
	ctx context.Context,
	codec keys.SQLCodec,
	table catalog.TableDescriptor,
	sourceID descpb.IndexID,
	destinationID descpb.IndexID,
	sourceSpans []roachpb.Span,
	readAsOf hlc.Timestamp,
) ([]roachpb.Span, error) {
	var completedSpans []roachpb.Span

	for i := range sourceSpans {
		nextKey, err := ibm.mergeSpan(ctx, codec, table, sourceID, destinationID, sourceSpans[i], readAsOf)
		if nextKey != nil {
			completedSpans = append(completedSpans, roachpb.Span{Key: sourceSpans[i].Key, EndKey: nextKey})
		} else {
			completedSpans = append(completedSpans, sourceSpans[i])
		}

		if err != nil {
			return completedSpans, err
		}
	}

	return completedSpans, nil
}

func (ibm *IndexBackfillMerger) mergeSpan(
	ctx context.Context,
	codec keys.SQLCodec,
	table catalog.TableDescriptor,
	sourceID descpb.IndexID,
	destinationID descpb.IndexID,
	sourceSpan roachpb.Span,
	readAsOf hlc.Timestamp,
) (roachpb.Key, error) {
	sourcePrefix := rowenc.MakeIndexKeyPrefix(codec, table.GetID(), sourceID)
	prefixLen := len(sourcePrefix)
	destPrefix := rowenc.MakeIndexKeyPrefix(codec, table.GetID(), destinationID)

	const pageSize = 1000
	key := sourceSpan.Key
	destKey := make([]byte, len(destPrefix))

	for key != nil {
		err := ibm.flowCtx.Cfg.DB.Txn(ctx, func(ctx context.Context, txn *kv.Txn) error {
			// For now just grab all of the destination KVs and merge the corresponding entries.
			kvs, err := txn.Scan(ctx, key, sourceSpan.EndKey, int64(pageSize))
			if err != nil {
				return err
			}

			if len(kvs) == 0 {
				key = nil
				return nil
			}

			destKeys := make([]roachpb.Key, len(kvs))
			for i := range kvs {
				sourceKV := &kvs[i]

				if len(sourceKV.Key) < prefixLen {
					return errors.Errorf("Key for index entry %v does not start with prefix %v", sourceKV, sourceSpan.Key)
				}

				destKey = destKey[:0]
				destKey = append(destKey, destPrefix...)
				destKey = append(destKey, sourceKV.Key[prefixLen:]...)
				destKeys[i] = make([]byte, len(destKey))
				copy(destKeys[i], destKey)
			}

			wb := txn.NewBatch()
			for i := range kvs {
				if kvs[i].Value.Timestamp.Less(readAsOf) {
					continue
				}
				mergedEntry, deleted, err := mergeEntry(&kvs[i], destKeys[i])
				if err != nil {
					return err
				}

				if deleted {
					wb.Del(mergedEntry.Key)
				} else {
					wb.Put(mergedEntry.Key, mergedEntry.Value)
				}
			}

			if err := txn.Run(ctx, wb); err != nil {
				return err
			}

			key = kvs[len(kvs)-1].Key.Next()
			return nil
		})

		if err != nil {
			return key, err
		}
	}

	return nil, nil
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
		Key:   destKey,
		Value: value,
	}, deleted, nil
}

func NewIndexBackfillMerger(
	flowCtx *execinfra.FlowCtx,
	spec execinfrapb.IndexBackfillMergerSpec,
	output execinfra.RowReceiver,
) (*IndexBackfillMerger, error) {
	im := &IndexBackfillMerger{
		spec:    spec,
		desc:    tabledesc.NewUnsafeImmutable(&spec.Table),
		flowCtx: flowCtx,

		evalCtx: flowCtx.NewEvalCtx(),
		output:  output,
	}

	return im, nil
}
