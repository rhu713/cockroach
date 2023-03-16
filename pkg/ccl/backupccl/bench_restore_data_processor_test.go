package backupccl

import (
	"context"
	"fmt"
	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/blobs"
	"github.com/cockroachdb/cockroach/pkg/ccl/backupccl/backuptestutils"
	"github.com/cockroachdb/cockroach/pkg/cloud"
	"github.com/cockroachdb/cockroach/pkg/cloud/cloudpb"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descs"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfra"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/util/ctxgroup"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"testing"
	"time"
)

func BenchmarkOpenSSTs(b *testing.B) {
	defer leaktest.AfterTest(b)()

	ctx := context.Background()

	tc, _, _, cleanupFn := backuptestutils.BackupRestoreTestSetup(b, backuptestutils.SingleNode, 1, backuptestutils.InitManualReplication)
	defer cleanupFn()
	s := tc.Server(0)

	//evalCtx := eval.Context{Settings: s.ClusterSettings(), Tracer: s.AmbientCtx().Tracer}
	flowCtx := execinfra.FlowCtx{
		Cfg: &execinfra.ServerConfig{
			DB: s.InternalDB().(descs.DB),
			ExternalStorage: func(ctx context.Context, dest cloudpb.ExternalStorage, opts ...cloud.ExternalStorageOption) (cloud.ExternalStorage, error) {
				return cloud.MakeExternalStorage(ctx, dest, base.ExternalIODirConfig{},
					s.ClusterSettings(), blobs.TestBlobServiceClient(s.ClusterSettings().ExternalIODir),
					nil, /* db */
					nil, /* limiters */
					cloud.NilMetrics,
					opts...)
			},
			Settings: s.ClusterSettings(),
			Codec:    keys.SystemSQLCodec,
		},
		EvalCtx: &eval.Context{
			Codec:    keys.SystemSQLCodec,
			Settings: s.ClusterSettings(),
		},
	}

	mockRestoreTime := hlc.Timestamp{
		WallTime: timeutil.Now().UnixNano(),
	}

	// TODO: add encryption to this bench as well.
	rd := restoreDataProcessor{
		spec:    execinfrapb.RestoreDataSpec{RestoreTime: mockRestoreTime},
		flowCtx: &flowCtx,
	}

	d := "s3://cockroach-fixtures/backups/tpc-e/customers=25000/v22.2.0/inc-count=48/2022/12/20-225543.90?AUTH=implicit"
	file := "data/824137976759058433.sst"
	for _, numIteratorFiles := range []int{3, 6, 12, 25, 50, 100, 200, 400, 800, 1600} {
		for _, numIters := range []int{3, 6, 12, 25, 50, 100, 200, 400, 800, 1600} {
			b.Run(fmt.Sprintf("numIters=%d/numIterFiles=%d", numIters, numIteratorFiles), func(b *testing.B) {
				sstCh := make(chan mergedSST, numIters) // Large enough to never block.
				entry := execinfrapb.RestoreSpanEntry{
					Span:        roachpb.Span{[]byte("a"), []byte("z")},
					Files:       nil,
					ProgressIdx: 0,
				}
				for i := 0; i < numIteratorFiles; i++ {
					conf, err := cloud.ExternalStorageConfFromURI(d, username.RootUserName())
					if err != nil {
						b.Fatal(err)
					}
					f := execinfrapb.RestoreFileSpec{
						Dir:  conf,
						Path: file,
					}
					entry.Files = append(entry.Files, f)
				}

				every5Sec := log.Every(5 * time.Second)
				g := ctxgroup.WithContext(ctx)
				g.GoCtx(func(ctx context.Context) error {
					for e := range sstCh {
						e.cleanup()
					}
					return nil
				})

				startTime := timeutil.Now()
				b.ResetTimer()
				for i := 0; i < numIters; i++ {
					if err := rd.openSSTs(ctx, entry, sstCh); err != nil {
						b.Fatal(err)
					}
					if every5Sec.ShouldLog() {
						log.Infof(ctx, "opened %d of %d SST iterators", i+1, numIters)
					}
				}
				b.StopTimer()
				duration := timeutil.Since(startTime)
				b.ReportMetric(float64(duration.Nanoseconds()/int64(numIters))/1_000_000.0, "ms/openSST")

				close(sstCh)
				if err := g.Wait(); err != nil {
					b.Fatal(err)
				}
			})
		}
	}
}
