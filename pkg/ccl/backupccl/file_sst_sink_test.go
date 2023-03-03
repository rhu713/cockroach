package backupccl

import (
	"context"
	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/blobs"
	"github.com/cockroachdb/cockroach/pkg/ccl/backupccl/backuppb"
	"github.com/cockroachdb/cockroach/pkg/cloud"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestOutOfOrderTimestampFlush(t *testing.T) {
	ctx := context.Background()
	tc, sqlDB, tmpDir, cleanupFn := backupRestoreTestSetup(t, singleNode, 1, InitManualReplication)
	store, err := cloud.ExternalStorageFromURI(ctx, "userfile:///0",
		base.ExternalIODirConfig{},
		tc.Servers[0].ClusterSettings(),
		blobs.TestEmptyBlobClientFactory,
		username.RootUserName(),
		tc.Servers[0].InternalDB().(isql.DB),
		nil, /* limiters */
		cloud.NilMetrics,
	)
	require.NoError(t, err)

	// Never block.
	progCh := make(chan execinfrapb.RemoteProducerMetadata_BulkProcessorProgress, 100)

	sinkConf := sstSinkConf{
		id:       1,
		enc:      nil,
		progCh:   progCh,
		settings: &tc.Servers[0].ClusterSettings().SV,
	}

	sink := makeFileSSTSink(sinkConf, store)

	sp1 := exportedSpan{
		metadata: backuppb.BackupManifest_File{
			Span:        roachpb.Span{},
			Path:        "",
			EntryCounts: roachpb.RowCount{},
			StartTime:   hlc.Timestamp{},
			EndTime:     hlc.Timestamp{},
			LocalityKV:  "",
		},
		dataSST:        nil,
		revStart:       hlc.Timestamp{},
		completedSpans: 0,
		atKeyBoundary:  false,
	}

}
