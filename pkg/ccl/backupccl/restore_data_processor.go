// Copyright 2020 The Cockroach Authors.
//
// Licensed as a CockroachDB Enterprise file under the Cockroach Community
// License (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     https://github.com/cockroachdb/cockroach/blob/master/licenses/CCL.txt

package backupccl

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"runtime"

	"github.com/cockroachdb/cockroach/pkg/ccl/backupccl/backuppb"
	"github.com/cockroachdb/cockroach/pkg/ccl/storageccl"
	"github.com/cockroachdb/cockroach/pkg/cloud"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv/bulk"
	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfra"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc"
	"github.com/cockroachdb/cockroach/pkg/sql/rowexec"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/storage"
	"github.com/cockroachdb/cockroach/pkg/util"
	bulkutil "github.com/cockroachdb/cockroach/pkg/util/bulk"
	"github.com/cockroachdb/cockroach/pkg/util/ctxgroup"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/mon"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
	"github.com/cockroachdb/cockroach/pkg/util/quotapool"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/logtags"
	gogotypes "github.com/gogo/protobuf/types"
)

// Progress is streamed to the coordinator through metadata.
var restoreDataOutputTypes = []*types.T{}

type restoreDataProcessor struct {
	execinfra.ProcessorBase

	flowCtx *execinfra.FlowCtx
	spec    execinfrapb.RestoreDataSpec
	input   execinfra.RowSource
	output  execinfra.RowReceiver

	// phaseGroup manages the phases of the restore:
	// 1) reading entries from the input
	// 2) ingesting the data associated with those entries in the concurrent
	// restore data workers.
	phaseGroup           ctxgroup.Group
	cancelWorkersAndWait func()

	// sstCh is a channel that holds SSTs opened by the processor, but not yet
	// ingested.
	sstCh chan mergedSST
	// Metas from the input are forwarded to the output of this processor.
	metaCh chan *execinfrapb.ProducerMetadata
	// progress updates are accumulated on this channel. It is populated by the
	// concurrent workers and sent down the flow by the processor.
	progCh chan backuppb.RestoreProgress

	agg *bulkutil.TracingAggregator

	mon             *mon.BytesMonitor
	mem             *mon.BoundAccount
	sstMemQuotaPool *quotapool.IntPool

	workerSSTCh []chan mergedSST
}

var (
	_ execinfra.Processor = &restoreDataProcessor{}
	_ execinfra.RowSource = &restoreDataProcessor{}
)

const restoreDataProcName = "restoreDataProcessor"

const maxConcurrentRestoreWorkers = 32

// sstReaderOverheadBytesPerFile and sstReaderEncryptedOverheadBytesPerFile were obtained
// benchmarking external SST iterators on GCP and AWS and selecting the highest
// observed memory per file.
const sstReaderOverheadBytesPerFile = 5 << 20
const sstReaderEncryptedOverheadBytesPerFile = 8 << 20

// minWorkerMemReservation is the minimum amount of memory reserved per restore
// data processor worker. It should be greater than
// sstReaderOverheadBytesPerFile and sstReaderEncryptedOverheadBytesPerFile to
// ensure that all workers at least can simultaneously process at least one
// file.
const minWorkerMemReservation = 25 << 20

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var defaultNumWorkers = util.ConstantWithMetamorphicTestRange(
	"restore-worker-concurrency",
	func() int {
		// On low-CPU instances, a default value may still allow concurrent restore
		// workers to tie up all cores so cap default value at cores-1 when the
		// default value is higher.
		restoreWorkerCores := runtime.GOMAXPROCS(0) - 1
		if restoreWorkerCores < 1 {
			restoreWorkerCores = 1
		}
		return min(4, restoreWorkerCores)
	}(), /* defaultValue */
	1, /* metamorphic min */
	8, /* metamorphic max */
)

// TODO(pbardea): It may be worthwhile to combine this setting with the one that
// controls the number of concurrent AddSSTable requests if each restore worker
// spends all if its time sending AddSSTable requests.
//
// The maximum is not enforced since if the maximum is reduced in the future that
// may cause the cluster setting to fail.
var numRestoreWorkers = settings.RegisterIntSetting(
	settings.TenantWritable,
	"kv.bulk_io_write.restore_node_concurrency",
	fmt.Sprintf("the number of workers processing a restore per job per node; maximum %d",
		maxConcurrentRestoreWorkers),
	int64(defaultNumWorkers),
	settings.PositiveInt,
)

// restorePerProcessorMemoryLimit is the limit on the memory used by a
// restoreDataProcessor.
var restorePerProcessorMemoryLimit = settings.RegisterByteSizeSetting(
	settings.TenantWritable,
	"bulkio.restore.per_processor_memory_limit",
	"limit on the amount of memory that can be used by a restore processor",
	512<<20, // 512 MiB
)

func newRestoreDataProcessor(
	ctx context.Context,
	flowCtx *execinfra.FlowCtx,
	processorID int32,
	spec execinfrapb.RestoreDataSpec,
	post *execinfrapb.PostProcessSpec,
	input execinfra.RowSource,
	output execinfra.RowReceiver,
) (execinfra.Processor, error) {
	rd := &restoreDataProcessor{
		flowCtx: flowCtx,
		input:   input,
		spec:    spec,
		output:  output,
		progCh:  make(chan backuppb.RestoreProgress, maxConcurrentRestoreWorkers),
		metaCh:  make(chan *execinfrapb.ProducerMetadata, 1),
	}

	rd.sstMemQuotaPool = quotapool.NewIntPool("restore-processor-sst-mem", 0)
	if spec.MemoryMonitorSSTs {
		limit := restorePerProcessorMemoryLimit.Get(&flowCtx.EvalCtx.Settings.SV)
		rd.mon = mon.NewMonitorInheritWithLimit("restore-processor-mon", limit, flowCtx.Cfg.RestoreMonitor)
		rd.mon.StartNoReserved(ctx, flowCtx.Cfg.RestoreMonitor)
		mem := rd.mon.MakeBoundAccount()
		mem.Mu = &syncutil.Mutex{}
		rd.mem = &mem
	}

	if err := rd.Init(ctx, rd, post, restoreDataOutputTypes, flowCtx, processorID, output, nil, /* memMonitor */
		execinfra.ProcStateOpts{
			InputsToDrain: []execinfra.RowSource{input},
			TrailingMetaCallback: func() []execinfrapb.ProducerMetadata {
				rd.ConsumerClosed()
				return nil
			},
		}); err != nil {
		return nil, err
	}
	return rd, nil
}

// Start is part of the RowSource interface.
func (rd *restoreDataProcessor) Start(ctx context.Context) {
	ctx = logtags.AddTag(ctx, "job", rd.spec.JobID)
	ctx = rd.StartInternal(ctx, restoreDataProcName)
	rd.input.Start(ctx)

	// First we reserve minWorkerMemReservation for each restore worker, and
	// making sure that we always have enough memory for at least one worker. The
	// maximum number of workers is based on the cluster setting. If the cluster
	// setting is updated, the job should be PAUSEd and RESUMEd for the new
	// setting to take effect.
	numWorkers, releaseWorkerMem, err := reserveRestoreWorkerMemory(ctx, rd.flowCtx.Cfg.Settings, rd.mem, rd.sstMemQuotaPool)
	if err != nil {
		rd.MoveToDraining(err)
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	ctx, rd.agg = bulkutil.MakeTracingAggregatorWithSpan(ctx, fmt.Sprintf("%s-aggregator", restoreDataProcName), rd.EvalCtx.Tracer)

	rd.cancelWorkersAndWait = func() {
		cancel()
		_ = rd.phaseGroup.Wait()
	}
	rd.phaseGroup = ctxgroup.WithContext(ctx)
	log.Infof(ctx, "starting restore data processor")

	entries := make(chan execinfrapb.RestoreSpanEntry, numWorkers)
	rd.sstCh = make(chan mergedSST, numWorkers)
	rd.workerSSTCh = make([]chan mergedSST, numWorkers)
	for i := range rd.workerSSTCh {
		rd.workerSSTCh[i] = make(chan mergedSST)
	}
	rd.phaseGroup.GoCtx(func(ctx context.Context) error {
		defer close(entries)
		return inputReader(ctx, rd.input, entries, rd.metaCh)
	})

	rd.phaseGroup.GoCtx(func(ctx context.Context) error {
		cleanup := func() {
			close(rd.sstCh)
			for _, ch := range rd.workerSSTCh {
				close(ch)
			}
		}
		defer cleanup()

		for entry := range entries {
			if err := rd.openSSTs(ctx, entry, rd.sstCh, rd.workerSSTCh); err != nil {
				return err
			}
		}

		return nil
	})

	rd.phaseGroup.GoCtx(func(ctx context.Context) error {
		defer releaseWorkerMem()
		defer close(rd.progCh)
		return rd.runRestoreWorkers(ctx, rd.sstCh, rd.workerSSTCh)
	})
}

// inputReader reads the rows from its input in a single thread and converts the
// rows into either `entries` which are passed to the restore workers or
// ProducerMetadata which is passed to `Next`.
//
// The contract of Next does not guarantee that the EncDatumRow returned by Next
// remains valid after the following call to Next. This is why the input is
// consumed on a single thread, rather than consumed by each worker.
func inputReader(
	ctx context.Context,
	input execinfra.RowSource,
	entries chan execinfrapb.RestoreSpanEntry,
	metaCh chan *execinfrapb.ProducerMetadata,
) error {
	var alloc tree.DatumAlloc

	for {
		// We read rows from the SplitAndScatter processor. We expect each row to
		// contain 2 columns. The first is used to route the row to this processor,
		// and the second contains the RestoreSpanEntry that we're interested in.
		row, meta := input.Next()
		if meta != nil {
			if meta.Err != nil {
				return meta.Err
			}

			select {
			case metaCh <- meta:
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}

		if row == nil {
			// Consumed all rows.
			return nil
		}

		if len(row) != 2 {
			return errors.New("expected input rows to have exactly 2 columns")
		}
		if err := row[1].EnsureDecoded(types.Bytes, &alloc); err != nil {
			return err
		}
		datum := row[1].Datum
		entryDatumBytes, ok := datum.(*tree.DBytes)
		if !ok {
			return errors.AssertionFailedf(`unexpected datum type %T: %+v`, datum, row)
		}

		var entry execinfrapb.RestoreSpanEntry
		if err := protoutil.Unmarshal([]byte(*entryDatumBytes), &entry); err != nil {
			return errors.Wrap(err, "un-marshaling restore span entry")
		}

		select {
		case entries <- entry:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

type mergedSST struct {
	entry      execinfrapb.RestoreSpanEntry
	iter       *storage.ReadAsOfIterator
	cleanup    func()
	lastInSpan bool
}

// openSSTs opens all files in entry and sends a multiplexed SST iterator over
// the files to sstCh. If memory monitoring is enabled and opening an additional
// file would exceed the current memory budget, a partial iterator over only the
// currently opened files would be sent first, with the intention of sending
// iterators over the remaining files when there's more memory available.
//
// NB: Since KVs in SSTables created by restore all have timestamps at the time
// of flush, it is important that the iterators be flushed in the same order as
// the backup layers (i.e. iterator containing the base backup must be flushed
// first). Therefore, all partial iterators over the same span must be processed
// and flushed by the same worker. workerSSTChannels exist so that there is a way
// to ensure the same worker gets a specific iterator.
func (rd *restoreDataProcessor) openSSTs(
	ctx context.Context,
	entry execinfrapb.RestoreSpanEntry,
	sstCh chan mergedSST,
	workerSSTChannels []chan mergedSST,
) error {
	ctxDone := ctx.Done()

	// TODO(msbutler): use a a map of external storage factories to avoid reopening the same dir
	// in a given restore span entry
	var dirs []cloud.ExternalStorage

	// If we bail early and haven't handed off responsibility of the dirs/iters to
	// the channel, close anything that we had open.
	defer func() {
		for _, dir := range dirs {
			if err := dir.Close(); err != nil {
				log.Warningf(ctx, "close export storage failed %v", err)
			}
		}
	}()

	// sendIter sends a multiplexed iterator covering the currently accumulated files over the
	// channel.
	sendIter := func(iter storage.SimpleMVCCIterator, dirsToSend []cloud.ExternalStorage, mSSTCh chan mergedSST, iterAllocs []*quotapool.IntAlloc, lastInSpan bool) error {
		readAsOfIter := storage.NewReadAsOfIterator(iter, rd.spec.RestoreTime)

		cleanup := func() {
			readAsOfIter.Close()
			rd.sstMemQuotaPool.Release(iterAllocs...)

			for _, dir := range dirsToSend {
				if err := dir.Close(); err != nil {
					log.Warningf(ctx, "close export storage failed %v", err)
				}
			}
		}

		mSST := mergedSST{
			entry:      entry,
			iter:       readAsOfIter,
			cleanup:    cleanup,
			lastInSpan: lastInSpan,
		}

		select {
		case mSSTCh <- mSST:
		case <-ctxDone:
			return ctx.Err()
		}

		dirs = make([]cloud.ExternalStorage, 0)
		return nil
	}

	hashSpan := func(span roachpb.Span) (uint32, error) {
		hash := fnv.New32a()
		_, err := hash.Write(span.Key)
		if err != nil {
			return 0, err
		}

		return hash.Sum32() % uint32(len(workerSSTChannels)), nil
	}

	sendWorkerSpecificIter := func(iter storage.SimpleMVCCIterator, dirsToSend []cloud.ExternalStorage, iterAllocs []*quotapool.IntAlloc, lastInSpan bool) error {
		workerNum, err := hashSpan(entry.Span)
		if err != nil {
			return err
		}

		return sendIter(iter, dirs, workerSSTChannels[workerNum], iterAllocs, lastInSpan)
	}

	log.VEventf(ctx, 1 /* level */, "ingesting span [%s-%s)", entry.Span.Key, entry.Span.EndKey)

	storeFiles := make([]storageccl.StoreFile, 0, len(entry.Files))
	iterAllocs := make([]*quotapool.IntAlloc, 0, len(entry.Files))
	var sstOverheadBytesPerFile uint64
	if rd.spec.Encryption != nil {
		sstOverheadBytesPerFile = sstReaderEncryptedOverheadBytesPerFile
	} else {
		sstOverheadBytesPerFile = sstReaderOverheadBytesPerFile
	}

	for idx := 0; idx < len(entry.Files); {
		file := entry.Files[idx]
		log.VEventf(ctx, 2, "import file %s which starts at %s", file.Path, entry.Span.Key)

		alloc, err := rd.sstMemQuotaPool.TryAcquire(ctx, sstOverheadBytesPerFile)
		if errors.Is(err, quotapool.ErrNotEnoughQuota) {
			if err := rd.mem.Grow(ctx, int64(sstOverheadBytesPerFile)); err != nil {
				// If we failed to allocate more memory, send the iterator
				// containing the files we have right now.
				if len(storeFiles) > 0 {
					iterOpts := storage.IterOptions{
						RangeKeyMaskingBelow: rd.spec.RestoreTime,
						KeyTypes:             storage.IterKeyTypePointsAndRanges,
						LowerBound:           keys.LocalMax,
						UpperBound:           keys.MaxKey,
					}
					iter, err := storageccl.ExternalSSTReader(ctx, storeFiles, rd.spec.Encryption, iterOpts)
					if err != nil {
						return err
					}

					log.VInfof(ctx, 2, "sending iterator after %d out of %d files due to insufficient memory", idx, len(entry.Files))
					err = sendWorkerSpecificIter(iter, dirs, iterAllocs, false)
					if err != nil {
						return err
					}

					storeFiles = make([]storageccl.StoreFile, 0, len(entry.Files)-idx)
					iterAllocs = make([]*quotapool.IntAlloc, 0, len(entry.Files)-idx)
				}

				alloc, err = rd.sstMemQuotaPool.Acquire(ctx, sstOverheadBytesPerFile)
				if err != nil {
					return err
				}
			} else {
				rd.sstMemQuotaPool.UpdateCapacity(rd.sstMemQuotaPool.Capacity() + sstOverheadBytesPerFile)
				// After updating the capacity, the TryAcquire should succeed. Thus we
				// return if we fail to acquire.
				alloc, err = rd.sstMemQuotaPool.TryAcquire(ctx, sstOverheadBytesPerFile)
				if err != nil {
					return err
				}
			}
		} else if err != nil {
			return err
		}

		iterAllocs = append(iterAllocs, alloc)

		dir, err := rd.flowCtx.Cfg.ExternalStorage(ctx, file.Dir)
		if err != nil {
			return err
		}
		dirs = append(dirs, dir)
		storeFiles = append(storeFiles, storageccl.StoreFile{Store: dir, FilePath: file.Path})
		idx++
	}

	iterOpts := storage.IterOptions{
		RangeKeyMaskingBelow: rd.spec.RestoreTime,
		KeyTypes:             storage.IterKeyTypePointsAndRanges,
		LowerBound:           keys.LocalMax,
		UpperBound:           keys.MaxKey,
	}
	iter, err := storageccl.ExternalSSTReader(ctx, storeFiles, rd.spec.Encryption, iterOpts)
	if err != nil {
		return err
	}

	if len(storeFiles) != len(entry.Files) {
		return sendWorkerSpecificIter(iter, dirs, iterAllocs, true)
	}
	return sendIter(iter, dirs, sstCh, iterAllocs, true)
}

func (rd *restoreDataProcessor) runRestoreWorkers(
	ctx context.Context, ssts chan mergedSST, workerSSTChannels []chan mergedSST,
) error {
	return ctxgroup.GroupWorkers(ctx, len(workerSSTChannels), func(ctx context.Context, worker int) error {
		kr, err := MakeKeyRewriterFromRekeys(rd.FlowCtx.Codec(), rd.spec.TableRekeys, rd.spec.TenantRekeys,
			false /* restoreTenantFromStream */)
		if err != nil {
			return err
		}

		ctx, agg := bulkutil.MakeTracingAggregatorWithSpan(ctx,
			fmt.Sprintf("%s-worker-%d-aggregator", restoreDataProcName, worker), rd.EvalCtx.Tracer)
		defer agg.Close()

		ssts := ssts
		workerSSTs := workerSSTChannels[worker]
		var sstIter mergedSST
		var ok bool

		for {
			done, err := func() (done bool, _ error) {
				select {
				case sstIter, ok = <-ssts:
					if !ok {
						ssts = nil
					}
				case sstIter, ok = <-workerSSTs:
					if !ok {
						workerSSTs = nil
					}
				case <-ctx.Done():
					return done, ctx.Err()
				}

				if ssts == nil && workerSSTs == nil {
					done = true
					return done, nil
				}

				if !ok {
					return done, nil
				}

				summary, err := rd.processRestoreSpanEntry(ctx, kr, sstIter)
				if err != nil {
					return done, err
				}

				select {
				case rd.progCh <- makeProgressUpdate(summary, sstIter.entry, rd.spec.PKIDs, rd.spec.RestoreTime, sstIter.lastInSpan):
				case <-ctx.Done():
					return done, ctx.Err()
				}

				return done, nil
			}()
			if err != nil {
				return err
			}

			if done {
				return nil
			}
		}
	})
}

func (rd *restoreDataProcessor) processRestoreSpanEntry(
	ctx context.Context, kr *KeyRewriter, sst mergedSST,
) (kvpb.BulkOpSummary, error) {
	db := rd.flowCtx.Cfg.DB
	evalCtx := rd.EvalCtx
	var summary kvpb.BulkOpSummary

	entry := sst.entry
	iter := sst.iter
	defer sst.cleanup()

	var batcher SSTBatcherExecutor
	if rd.spec.ValidateOnly {
		batcher = &sstBatcherNoop{}
	} else {
		// If the system tenant is restoring a guest tenant span, we don't want to
		// forward all the restored data to now, as there may be importing tables in
		// that span, that depend on the difference in timestamps on restored existing
		// vs importing keys to rollback.
		writeAtBatchTS := true
		if writeAtBatchTS && kr.fromSystemTenant &&
			(bytes.HasPrefix(entry.Span.Key, keys.TenantPrefix) || bytes.HasPrefix(entry.Span.EndKey, keys.TenantPrefix)) {
			log.Warningf(ctx, "restoring span %s at its original timestamps because it is a tenant span", entry.Span)
			writeAtBatchTS = false
		}

		// disallowShadowingBelow is set to an empty hlc.Timestamp in release builds
		// i.e. allow all shadowing without AddSSTable having to check for
		// overlapping keys. This is necessary since RESTORE can sometimes construct
		// SSTables that overwrite existing keys, in cases when there wasn't
		// sufficient memory to open an iterator for all files at once for a given
		// import span.
		//
		// NB: disallowShadowingBelow used to be unconditionally set to logical=1.
		// This permissive value would allow shadowing in case the RESTORE has to
		// retry ingestions but served to force evaluation of AddSSTable to check for
		// overlapping keys. It was believed that even across resumptions of a restore
		// job, `checkForKeyCollisions` would be inexpensive because of our frequent
		// job checkpointing. Further investigation in
		// https://github.com/cockroachdb/cockroach/issues/81116 revealed that our
		// progress checkpointing could significantly lag behind the spans we have
		// ingested, making a resumed restore spend a lot of time in
		// `checkForKeyCollisions` leading to severely degraded performance. We have
		// *never* seen a restore fail because of the invariant enforced by setting
		// `disallowShadowingBelow` to a non-empty value, and so we feel comfortable
		// disabling this check entirely. A future release will work on fixing our
		// progress checkpointing so that we do not have a buildup of un-checkpointed
		// work, at which point we can reassess reverting to logical=1.
		disallowShadowingBelow := hlc.Timestamp{}

		var err error
		batcher, err = bulk.MakeSSTBatcher(ctx,
			"restore",
			db.KV(),
			evalCtx.Settings,
			disallowShadowingBelow,
			writeAtBatchTS,
			false, /* scatterSplitRanges */
			// TODO(rui): we can change this to the processor's bound account, but
			// currently there seems to be some accounting errors that will cause
			// tests to fail.
			rd.flowCtx.Cfg.BackupMonitor.MakeBoundAccount(),
			rd.flowCtx.Cfg.BulkSenderLimiter,
		)
		if err != nil {
			return summary, err
		}
	}
	defer batcher.Close(ctx)

	// Read log.V once first to avoid the vmodule mutex in the tight loop below.
	verbose := log.V(5)

	var keyScratch, valueScratch []byte

	startKeyMVCC, endKeyMVCC := storage.MVCCKey{Key: entry.Span.Key},
		storage.MVCCKey{Key: entry.Span.EndKey}

	for iter.SeekGE(startKeyMVCC); ; iter.NextKey() {
		ok, err := iter.Valid()
		if err != nil {
			return summary, err
		}

		if !ok || !iter.UnsafeKey().Less(endKeyMVCC) {
			break
		}

		key := iter.UnsafeKey()
		keyScratch = append(keyScratch[:0], key.Key...)
		key.Key = keyScratch
		v, err := iter.UnsafeValue()
		if err != nil {
			return summary, err
		}
		valueScratch = append(valueScratch[:0], v...)
		value := roachpb.Value{RawBytes: valueScratch}

		key.Key, ok, err = kr.RewriteKey(key.Key, key.Timestamp.WallTime)

		if errors.Is(err, ErrImportingKeyError) {
			// The keyRewriter returns ErrImportingKeyError iff the key is part of an
			// in-progress import. Keys from in-progress imports never get restored,
			// since the key's table gets restored to its pre-import state. Therefore,
			// elide ingesting this key.
			continue
		}
		if err != nil {
			return summary, err
		}
		if !ok {
			// If the key rewriter didn't match this key, it's not data for the
			// table(s) we're interested in.
			if verbose {
				log.Infof(ctx, "skipping %s %s", key.Key, value.PrettyPrint())
			}
			continue
		}

		// Rewriting the key means the checksum needs to be updated.
		value.ClearChecksum()
		value.InitChecksum(key.Key)

		if verbose {
			log.Infof(ctx, "Put %s -> %s", key.Key, value.PrettyPrint())
		}
		if err := batcher.AddMVCCKey(ctx, key, value.RawBytes); err != nil {
			return summary, errors.Wrapf(err, "adding to batch: %s -> %s", key, value.PrettyPrint())
		}
	}
	// Flush out the last batch.
	if err := batcher.Flush(ctx); err != nil {
		return summary, err
	}

	if restoreKnobs, ok := rd.flowCtx.TestingKnobs().BackupRestoreTestingKnobs.(*sql.BackupRestoreTestingKnobs); ok {
		if restoreKnobs.RunAfterProcessingRestoreSpanEntry != nil {
			restoreKnobs.RunAfterProcessingRestoreSpanEntry(ctx, &entry)
		}
	}

	return batcher.GetSummary(), nil
}

func makeProgressUpdate(
	summary kvpb.BulkOpSummary,
	entry execinfrapb.RestoreSpanEntry,
	pkIDs map[uint64]bool,
	restoreTime hlc.Timestamp,
	lastInSpan bool,
) (progDetails backuppb.RestoreProgress) {
	progDetails.Summary = countRows(summary, pkIDs)
	progDetails.ProgressIdx = entry.ProgressIdx
	progDetails.DataSpan = entry.Span
	if !lastInSpan {
		// TODO(rui): this is a placeholder value to show that a span has been
		// partially but not completely processed. Eventually this timestamp should
		// be the actual timestamp that we have processed up to so far.
		progDetails.CompleteUpTo = hlc.Timestamp{WallTime: 1}
	} else {
		progDetails.CompleteUpTo = restoreTime
	}
	return
}

// Next is part of the RowSource interface.
func (rd *restoreDataProcessor) Next() (rowenc.EncDatumRow, *execinfrapb.ProducerMetadata) {
	if rd.State != execinfra.StateRunning {
		return nil, rd.DrainHelper()
	}

	var prog execinfrapb.RemoteProducerMetadata_BulkProcessorProgress
	select {
	case progDetails, ok := <-rd.progCh:
		if !ok {
			// Done. Check if any phase exited early with an error.
			err := rd.phaseGroup.Wait()
			rd.MoveToDraining(err)
			return nil, rd.DrainHelper()
		}

		details, err := gogotypes.MarshalAny(&progDetails)
		if err != nil {
			rd.MoveToDraining(err)
			return nil, rd.DrainHelper()
		}
		prog.ProgressDetails = *details
	case meta := <-rd.metaCh:
		return nil, meta
	case <-rd.Ctx().Done():
		rd.MoveToDraining(rd.Ctx().Err())
		return nil, rd.DrainHelper()
	}

	return nil, &execinfrapb.ProducerMetadata{BulkProcessorProgress: &prog}
}

// ConsumerClosed is part of the RowSource interface.
func (rd *restoreDataProcessor) ConsumerClosed() {
	if rd.Closed {
		return
	}
	rd.cancelWorkersAndWait()
	if rd.sstCh != nil {
		// Cleanup all the remaining open SSTs that have not been consumed.
		for sst := range rd.sstCh {
			sst.cleanup()
		}
	}

	for _, workerCh := range rd.workerSSTCh {
		if workerCh != nil {
			// Cleanup all the remaining open SSTs that have not been consumed.
			for sst := range rd.sstCh {
				sst.cleanup()
			}
		}
	}

	rd.mem.Close(rd.Ctx())
	if rd.mon != nil {
		rd.mon.Stop(rd.Ctx())
	}
	rd.agg.Close()
	rd.InternalClose()
}

func reserveRestoreWorkerMemory(
	ctx context.Context,
	settings *cluster.Settings,
	mem *mon.BoundAccount,
	quotaPool *quotapool.IntPool,
) (int, func(), error) {
	numWorkersSetting := int(numRestoreWorkers.Get(&settings.SV))

	numWorkers := 0
	for worker := 0; worker < numWorkersSetting; worker++ {
		if err := mem.Grow(ctx, minWorkerMemReservation); err != nil {
			if worker != 0 {
				break // no more memory to run workers
			}
			return 0, nil, errors.Wrap(err, "insufficient memory available to run restore")
		} else {
			quotaPool.UpdateCapacity(quotaPool.Capacity() + minWorkerMemReservation)
			numWorkers++
		}
	}

	cleanup := func() {
		mem.Shrink(ctx, int64(quotaPool.Capacity()))
	}

	return numWorkers, cleanup, nil
}

// SSTBatcherExecutor wraps the SSTBatcher methods, allowing a validation only restore to
// implement a mock SSTBatcher used purely for job progress tracking.
type SSTBatcherExecutor interface {
	AddMVCCKey(ctx context.Context, key storage.MVCCKey, value []byte) error
	Reset(ctx context.Context) error
	Flush(ctx context.Context) error
	Close(ctx context.Context)
	GetSummary() kvpb.BulkOpSummary
}

type sstBatcherNoop struct {
	// totalRows written by the batcher
	totalRows storage.RowCounter
}

var _ SSTBatcherExecutor = &sstBatcherNoop{}

// AddMVCCKey merely increments the totalRow Counter. No key gets buffered or written.
func (b *sstBatcherNoop) AddMVCCKey(ctx context.Context, key storage.MVCCKey, value []byte) error {
	return b.totalRows.Count(key.Key)
}

// Reset resets the counter
func (b *sstBatcherNoop) Reset(ctx context.Context) error {
	return nil
}

// Flush noops.
func (b *sstBatcherNoop) Flush(ctx context.Context) error {
	return nil
}

// Close noops.
func (b *sstBatcherNoop) Close(ctx context.Context) {
}

// GetSummary returns this batcher's total added rows/bytes/etc.
func (b *sstBatcherNoop) GetSummary() kvpb.BulkOpSummary {
	return b.totalRows.BulkOpSummary
}

func init() {
	rowexec.NewRestoreDataProcessor = newRestoreDataProcessor
}
