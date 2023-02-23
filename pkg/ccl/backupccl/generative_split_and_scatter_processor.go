// Copyright 2023 The Cockroach Authors.
//
// Licensed as a CockroachDB Enterprise file under the Cockroach Community
// License (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     https://github.com/cockroachdb/cockroach/blob/master/licenses/CCL.txt

package backupccl

import (
	"context"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"time"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/ccl/backupccl/backupencryption"
	"github.com/cockroachdb/cockroach/pkg/ccl/backupccl/backupinfo"
	"github.com/cockroachdb/cockroach/pkg/ccl/backupccl/backuppb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfra"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc"
	"github.com/cockroachdb/cockroach/pkg/sql/rowexec"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/ctxgroup"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/cockroachdb/logtags"
)

const generativeSplitAndScatterProcessorName = "generativeSplitAndScatter"

var generativeSplitAndScatterOutputTypes = []*types.T{
	types.Bytes, // Span key for the range router
	types.Bytes, // RestoreDataEntry bytes
}

var numSplitScatterWorkers = settings.RegisterIntSetting(
	settings.TenantWritable,
	"bulkio.restore.num_split_scatter_workers",
	"number of split and scatter workers. If set to 0, this will default to the number of nodes in the cluster.",
	1,
	settings.NonNegativeInt,
)

// generativeSplitAndScatterProcessor is given a backup chain, whose manifests
// are specified in URIs and iteratively generates RestoreSpanEntries to be
// distributed across the cluster. Depending on which node the span ends up on,
// it forwards RestoreSpanEntry as bytes along with the key of the span on a
// row. It expects an output RangeRouter and before it emits each row, it
// updates the entry in the RangeRouter's map with the destination of the
// scatter.
type generativeSplitAndScatterProcessor struct {
	execinfra.ProcessorBase

	flowCtx *execinfra.FlowCtx
	spec    execinfrapb.GenerativeSplitAndScatterSpec
	output  execinfra.RowReceiver

	// chunkSplitAndScatterers contain the splitAndScatterers for the group of
	// split and scatter workers that's responsible for splitting and scattering
	// the import span chunks. Each worker needs its own scatterer as one cannot
	// be used concurrently.
	chunkSplitAndScatterers []splitAndScatterer
	// chunkEntrySplitAndScatterers contain the splitAndScatterers for the group of
	// split workers that's responsible for making splits at each import span
	// entry. These scatterers only create splits for the start key of each import
	// span and do not perform any scatters.
	chunkEntrySplitAndScatterers []splitAndScatterer

	// cancelScatterAndWaitForWorker cancels the scatter goroutine and waits for
	// it to finish.
	cancelScatterAndWaitForWorker func()

	doneScatterCh chan entryNode
	// A cache for routing datums, so only 1 is allocated per node.
	routingDatumCache map[roachpb.NodeID]rowenc.EncDatum
	cachedNodeIDs     []roachpb.NodeID
	scatterErr        error
}

var _ execinfra.Processor = &generativeSplitAndScatterProcessor{}

func newGenerativeSplitAndScatterProcessor(
	ctx context.Context,
	flowCtx *execinfra.FlowCtx,
	processorID int32,
	spec execinfrapb.GenerativeSplitAndScatterSpec,
	post *execinfrapb.PostProcessSpec,
	output execinfra.RowReceiver,
) (execinfra.Processor, error) {
	db := flowCtx.Cfg.DB
	numChunkSplitAndScatterWorkers := int(spec.NumNodes)
	if n := numSplitScatterWorkers.Get(&flowCtx.Cfg.Settings.SV); n > 0 {
		numChunkSplitAndScatterWorkers = int(n)
	}

	// numEntrySplitWorkers is set to be 2 * numChunkSplitAndScatterWorkers in
	// order to keep up with the rate at which chunks are split and scattered.
	// TODO(rui): This tries to cover for a bad scatter by having 2 * the number
	// of numChunkSplitAndScatterWorkers in the cluster. Does this knob need to be
	// re-tuned?
	numEntrySplitWorkers := 2 * numChunkSplitAndScatterWorkers

	mkSplitAndScatterer := func() (splitAndScatterer, error) {
		if spec.ValidateOnly {
			nodeID, _ := flowCtx.NodeID.OptionalNodeID()
			return noopSplitAndScatterer{nodeID}, nil
		}
		kr, err := MakeKeyRewriterFromRekeys(flowCtx.Codec(), spec.TableRekeys, spec.TenantRekeys,
			false /* restoreTenantFromStream */)
		if err != nil {
			return nil, err
		}
		return makeSplitAndScatterer(db.KV(), kr), nil
	}

	var chunkSplitAndScatterers []splitAndScatterer
	for i := 0; i < numChunkSplitAndScatterWorkers; i++ {
		scatterer, err := mkSplitAndScatterer()
		if err != nil {
			return nil, err
		}
		chunkSplitAndScatterers = append(chunkSplitAndScatterers, scatterer)
	}

	var chunkEntrySplitAndScatterers []splitAndScatterer
	for i := 0; i < numEntrySplitWorkers; i++ {
		scatterer, err := mkSplitAndScatterer()
		if err != nil {
			return nil, err
		}
		chunkEntrySplitAndScatterers = append(chunkEntrySplitAndScatterers, scatterer)
	}

	ssp := &generativeSplitAndScatterProcessor{
		flowCtx:                      flowCtx,
		spec:                         spec,
		output:                       output,
		chunkSplitAndScatterers:      chunkSplitAndScatterers,
		chunkEntrySplitAndScatterers: chunkEntrySplitAndScatterers,
		// Large enough so it never blocks.
		doneScatterCh:     make(chan entryNode, spec.NumEntries),
		routingDatumCache: make(map[roachpb.NodeID]rowenc.EncDatum),
	}
	if err := ssp.Init(ctx, ssp, post, generativeSplitAndScatterOutputTypes, flowCtx, processorID, output, nil, /* memMonitor */
		execinfra.ProcStateOpts{
			InputsToDrain: nil, // there are no inputs to drain
			TrailingMetaCallback: func() []execinfrapb.ProducerMetadata {
				ssp.close()
				return nil
			},
		}); err != nil {
		return nil, err
	}
	return ssp, nil
}

// Start is part of the RowSource interface.
func (gssp *generativeSplitAndScatterProcessor) Start(ctx context.Context) {
	ctx = logtags.AddTag(ctx, "job", gssp.spec.JobID)
	ctx = gssp.StartInternal(ctx, generativeSplitAndScatterProcessorName)
	// Note that the loop over doneScatterCh in Next should prevent the goroutine
	// below from leaking when there are no errors. However, if that loop needs to
	// exit early, runSplitAndScatter's context will be canceled.
	scatterCtx, cancel := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	gssp.cancelScatterAndWaitForWorker = func() {
		cancel()
		<-workerDone
	}
	if err := gssp.flowCtx.Stopper().RunAsyncTaskEx(scatterCtx, stop.TaskOpts{
		TaskName: "generativeSplitAndScatter-worker",
		SpanOpt:  stop.ChildSpan,
	}, func(ctx context.Context) {
		gssp.scatterErr = gssp.runGenerativeSplitAndScatter(scatterCtx)
		cancel()
		close(gssp.doneScatterCh)
		close(workerDone)
	}); err != nil {
		gssp.scatterErr = err
		cancel()
		close(workerDone)
	}
}

// Next implements the execinfra.RowSource interface.
func (gssp *generativeSplitAndScatterProcessor) Next() (
	rowenc.EncDatumRow,
	*execinfrapb.ProducerMetadata,
) {
	if gssp.State != execinfra.StateRunning {
		return nil, gssp.DrainHelper()
	}

	scatteredEntry, ok := <-gssp.doneScatterCh
	if ok {
		entry := scatteredEntry.entry
		entryBytes, err := protoutil.Marshal(&entry)
		if err != nil {
			gssp.MoveToDraining(err)
			return nil, gssp.DrainHelper()
		}

		// The routing datums informs the router which output stream should be used.
		routingDatum, ok := gssp.routingDatumCache[scatteredEntry.node]
		if !ok {
			routingDatum, _ = routingDatumsForSQLInstance(base.SQLInstanceID(scatteredEntry.node))
			gssp.routingDatumCache[scatteredEntry.node] = routingDatum
			gssp.cachedNodeIDs = append(gssp.cachedNodeIDs, scatteredEntry.node)
		}

		row := rowenc.EncDatumRow{
			routingDatum,
			rowenc.DatumToEncDatum(types.Bytes, tree.NewDBytes(tree.DBytes(entryBytes))),
		}
		return row, nil
	}

	if gssp.scatterErr != nil {
		gssp.MoveToDraining(gssp.scatterErr)
		return nil, gssp.DrainHelper()
	}

	gssp.MoveToDraining(nil /* error */)
	return nil, gssp.DrainHelper()
}

// ConsumerClosed is part of the RowSource interface.
func (gssp *generativeSplitAndScatterProcessor) ConsumerClosed() {
	// The consumer is done, Next() will not be called again.
	gssp.close()
}

// close stops the production workers. This needs to be called if the consumer
// runs into an error and stops consuming scattered entries to make sure we
// don't leak goroutines.
func (gssp *generativeSplitAndScatterProcessor) close() {
	gssp.cancelScatterAndWaitForWorker()
	gssp.InternalClose()
}

func makeBackupMetadata(
	ctx context.Context, flowCtx *execinfra.FlowCtx, spec *execinfrapb.GenerativeSplitAndScatterSpec,
) ([]backuppb.BackupManifest, layerToBackupManifestFileIterFactory, error) {

	execCfg := flowCtx.Cfg.ExecutorConfig.(*sql.ExecutorConfig)

	kmsEnv := backupencryption.MakeBackupKMSEnv(execCfg.Settings, &execCfg.ExternalIODirConfig,
		execCfg.InternalDB, spec.User())

	backupManifests, _, err := backupinfo.LoadBackupManifestsAtTime(ctx, nil, spec.URIs,
		spec.User(), execCfg.DistSQLSrv.ExternalStorageFromURI, spec.Encryption, &kmsEnv, spec.EndTime)
	if err != nil {
		return nil, nil, err
	}

	layerToBackupManifestFileIterFactory, err := getBackupManifestFileIters(ctx, execCfg,
		backupManifests, spec.Encryption, &kmsEnv)
	if err != nil {
		return nil, nil, err
	}

	return backupManifests, layerToBackupManifestFileIterFactory, nil
}

type restoreEntryChunk struct {
	entries  []execinfrapb.RestoreSpanEntry
	splitKey roachpb.Key
}

func (gssp *generativeSplitAndScatterProcessor) runGenerativeSplitAndScatter(
	ctx context.Context,
) error {
	log.Infof(ctx, "Running generative split and scatter with %d total spans, %d chunk size, %d nodes",
		gssp.spec.NumEntries, gssp.spec.ChunkSize, gssp.spec.NumNodes)
	g := ctxgroup.WithContext(ctx)

	chunkSplitAndScatterWorkers := len(gssp.chunkSplitAndScatterers)
	restoreSpanEntriesCh := make(chan execinfrapb.RestoreSpanEntry, chunkSplitAndScatterWorkers*int(gssp.spec.ChunkSize))

	// This goroutine generates import spans one at a time and sends them to
	// restoreSpanEntriesCh.
	g.GoCtx(func(ctx context.Context) error {
		defer close(restoreSpanEntriesCh)

		backups, layerToFileIterFactory, err := makeBackupMetadata(ctx,
			gssp.flowCtx, &gssp.spec)
		if err != nil {
			return err
		}

		introducedSpanFrontier, err := createIntroducedSpanFrontier(backups, gssp.spec.EndTime)
		if err != nil {
			return err
		}

		backupLocalityMap, err := makeBackupLocalityMap(gssp.spec.BackupLocalityInfo, gssp.spec.User())
		if err != nil {
			return err
		}

		return generateAndSendImportSpans(
			ctx,
			gssp.spec.Spans,
			backups,
			layerToFileIterFactory,
			backupLocalityMap,
			introducedSpanFrontier,
			gssp.spec.HighWater,
			gssp.spec.TargetSize,
			restoreSpanEntriesCh,
			gssp.spec.UseSimpleImportSpans,
		)
	})

	restoreEntryChunksCh := make(chan restoreEntryChunk, chunkSplitAndScatterWorkers)

	// This goroutine takes the import spans off of restoreSpanEntriesCh and
	// groups them into chunks of spec.ChunkSize. These chunks are then sent to
	// restoreEntryChunksCh.
	g.GoCtx(func(ctx context.Context) error {
		defer close(restoreEntryChunksCh)

		var idx int64
		var chunk restoreEntryChunk
		for entry := range restoreSpanEntriesCh {
			entry.ProgressIdx = idx
			idx++
			if len(chunk.entries) == int(gssp.spec.ChunkSize) {
				chunk.splitKey = entry.Span.Key
				select {
				case <-ctx.Done():
					return ctx.Err()
				case restoreEntryChunksCh <- chunk:
				}
				chunk = restoreEntryChunk{}
			}
			chunk.entries = append(chunk.entries, entry)
		}

		if len(chunk.entries) > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case restoreEntryChunksCh <- chunk:
			}
		}
		return nil
	})

	importSpanChunksCh := make(chan scatteredChunk, chunkSplitAndScatterWorkers*2)

	entriesByNode := make(map[roachpb.NodeID]int)
	filesByNode := make(map[roachpb.NodeID]int)

	ticker := time.NewTicker(5 * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C:
				log.Infof(ctx, "======\n\nentries=%v \n\nfiles=%v\n\n======", entriesByNode, filesByNode)
			case <-ctx.Done():
				return
			}
		}
	}()

	// This group of goroutines processes the chunks from restoreEntryChunksCh.
	// For each chunk, a split is created at the start key of the next chunk. The
	// current chunk is then scattered, and the chunk with its destination is
	// passed to importSpanChunksCh.
	g2 := ctxgroup.WithContext(ctx)
	for worker := 0; worker < chunkSplitAndScatterWorkers; worker++ {
		worker := worker
		g2.GoCtx(func(ctx context.Context) error {
			// Chunks' leaseholders should be randomly placed throughout the
			// cluster.
			for importSpanChunk := range restoreEntryChunksCh {
				scatterKey := importSpanChunk.entries[0].Span.Key
				if !importSpanChunk.splitKey.Equal(roachpb.Key{}) {
					// Split at the start of the next chunk, to partition off a
					// prefix of the space to scatter.
					if err := gssp.chunkSplitAndScatterers[worker].split(ctx, gssp.flowCtx.Codec(), importSpanChunk.splitKey); err != nil {
						return err
					}
				}
				chunkDestination, err := gssp.chunkSplitAndScatterers[worker].scatter(ctx, gssp.flowCtx.Codec(), scatterKey)
				if err != nil {
					return err
				}
				if chunkDestination == 0 {
					// If scatter failed to find a node for range ingestion, route the range
					// to a random node that's already been scattered to so far.
					if nodeID, ok := gssp.flowCtx.NodeID.OptionalNodeID(); ok {
						if len(gssp.cachedNodeIDs) > 0 && len(importSpanChunk.splitKey) > 0 {
							randomNum := int(importSpanChunk.splitKey[len(importSpanChunk.splitKey)-1])
							nodeID = gssp.cachedNodeIDs[randomNum%len(gssp.cachedNodeIDs)]

							log.Warningf(ctx, "scatter returned node 0. "+
								"Random route span starting at %s node %v", scatterKey, nodeID)
						} else {
							log.Warningf(ctx, "scatter returned node 0. "+
								"Route span starting at %s to current node %v", scatterKey, nodeID)
						}
						chunkDestination = nodeID
					} else {
						log.Warningf(ctx, "scatter returned node 0. "+
							"Route span starting at %s to default stream", scatterKey)
					}
				}

				sc := scatteredChunk{
					destination: chunkDestination,
					entries:     importSpanChunk.entries,
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case importSpanChunksCh <- sc:
				}
			}
			return nil
		})
	}

	// This goroutine waits for the chunkSplitAndScatter workers to finish so that
	// it can close importSpanChunksCh.
	g.GoCtx(func(ctx context.Context) error {
		defer close(importSpanChunksCh)
		err := g2.Wait()
		return err
	})

	// Large enough so it never blocks.
	unsortedDoneScatterCh := make(chan entryNode, gssp.spec.NumEntries)
	// This group of goroutines takes chunks that have already been split and
	// scattered by the previous worker group. These workers create splits at the
	// start key of the span of every entry of every chunk. After a chunk has been
	// processed, it is passed to doneScatterCh to signal that the chunk has gone
	// through the entire split and scatter process.
	// TODO: comment change
	g3 := ctxgroup.WithContext(ctx)
	for worker := 0; worker < len(gssp.chunkEntrySplitAndScatterers); worker++ {
		worker := worker
		g3.GoCtx(func(ctx context.Context) error {
			for importSpanChunk := range importSpanChunksCh {
				chunkDestination := importSpanChunk.destination
				for i, importEntry := range importSpanChunk.entries {
					nextChunkIdx := i + 1

					log.VInfof(ctx, 2, "processing a span [%s,%s) with destination %v idx %d", importEntry.Span.Key, importEntry.Span.EndKey, importSpanChunk.destination, importEntry.ProgressIdx)

					var splitKey roachpb.Key
					if nextChunkIdx < len(importSpanChunk.entries) {
						// Split at the next entry.
						log.VInfof(ctx, 2, "splitting a span [%s,%s) with destination %v idx %d", importEntry.Span.Key, importEntry.Span.EndKey, importSpanChunk.destination, importEntry.ProgressIdx)
						splitKey = importSpanChunk.entries[nextChunkIdx].Span.Key
						if err := gssp.chunkEntrySplitAndScatterers[worker].split(ctx, gssp.flowCtx.Codec(), splitKey); err != nil {
							log.VInfof(ctx, 2, "err splitting a span [%s,%s) with destination %v idx %d %v", importEntry.Span.Key, importEntry.Span.EndKey, importSpanChunk.destination, importEntry.ProgressIdx, err)
							return err
						}
						log.VInfof(ctx, 2, "done splitting a span [%s,%s) with destination %v idx %d", importEntry.Span.Key, importEntry.Span.EndKey, importSpanChunk.destination, importEntry.ProgressIdx)
					}

					scatteredEntry := entryNode{
						entry: importEntry,
						node:  chunkDestination,
					}

					if restoreKnobs, ok := gssp.flowCtx.TestingKnobs().BackupRestoreTestingKnobs.(*sql.BackupRestoreTestingKnobs); ok {
						if restoreKnobs.RunAfterSplitAndScatteringEntry != nil {
							restoreKnobs.RunAfterSplitAndScatteringEntry(ctx)
						}
					}

					log.VInfof(ctx, 2, "done processing a span [%s,%s) with destination %v idx %d", importEntry.Span.Key, importEntry.Span.EndKey, importSpanChunk.destination, importEntry.ProgressIdx)
					select {
					case <-ctx.Done():
						return ctx.Err()
					case unsortedDoneScatterCh <- scatteredEntry:
						log.VInfof(ctx, 2, "done sending a span [%s,%s) with destination %v idx %d", importEntry.Span.Key, importEntry.Span.EndKey, importSpanChunk.destination, importEntry.ProgressIdx)
						entriesByNode[scatteredEntry.node] += 1
						filesByNode[scatteredEntry.node] += len(scatteredEntry.entry.Files)
					}
				}
			}
			return nil
		})
	}

	g.GoCtx(func(ctx context.Context) error {
		defer close(unsortedDoneScatterCh)
		return g3.Wait()
	})

	mu := syncutil.Mutex{}
	doneScatteredEntries := make(map[int64]entryNode)
	g.GoCtx(func(ctx context.Context) error {
		for entry := range unsortedDoneScatterCh {
			log.VInfof(ctx, 2, "unsorted span [%s,%s) with idx %d", entry.entry.Span.Key, entry.entry.Span.EndKey, entry.entry.ProgressIdx)
			mu.Lock()
			doneScatteredEntries[entry.entry.ProgressIdx] = entry
			mu.Unlock()

			//if sendEntry, ok := doneScatteredEntries[nextIndex]; ok {
			//	select {
			//	case <-ctx.Done():
			//		return ctx.Err()
			//	case doneScatterCh <- sendEntry:
			//		log.VInfof(ctx, 2, "sending unsorted span [%s,%s) with idx %d", entry.entry.Span.Key, entry.entry.Span.EndKey, entry.entry.ProgressIdx)
			//		nextIndex++
			//		delete(doneScatteredEntries, nextIndex)
			//	}
			//}
		}
		return nil
	})

	g.GoCtx(func(ctx context.Context) error {
		var nextIndex int64
		for nextIndex < gssp.spec.NumEntries {
			mu.Lock()
			sendEntry, ok := doneScatteredEntries[nextIndex]
			if ok {
				log.VInfof(ctx, 2, "cleanup send span [%s,%s) with idx %d", sendEntry.entry.Span.Key, sendEntry.entry.Span.EndKey, sendEntry.entry.ProgressIdx)
				select {
				case <-ctx.Done():
					mu.Unlock()
					return ctx.Err()
				case gssp.doneScatterCh <- sendEntry:
					delete(doneScatteredEntries, nextIndex)
					nextIndex++
				}
				mu.Unlock()
			} else {
				log.Infof(ctx, "rh_debug: want to send idx %d map=%v\n", nextIndex, doneScatteredEntries)
				mu.Unlock()
				select {
				case <-time.After(time.Second):
					continue
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
		return nil
	})

	return g.Wait()
}

func init() {
	rowexec.NewGenerativeSplitAndScatterProcessor = newGenerativeSplitAndScatterProcessor
}
