package backupccl

import (
	"bytes"
	"context"
	"fmt"
	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/blobs"
	"github.com/cockroachdb/cockroach/pkg/ccl/backupccl/backuppb"
	"github.com/cockroachdb/cockroach/pkg/cloud"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/storage"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestOutOfOrderTimestampFlush(t *testing.T) {
	ctx := context.Background()
	tc, sqlDB, _, _ := backupRestoreTestSetup(t, singleNode, 1, InitManualReplication)
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
	sqlDB.Exec(t, `SET CLUSTER SETTING bulkio.backup.file_size = '20B'`)

	// Never block.
	progCh := make(chan execinfrapb.RemoteProducerMetadata_BulkProcessorProgress, 100)

	sinkConf := sstSinkConf{
		id:       1,
		enc:      nil,
		progCh:   progCh,
		settings: &tc.Servers[0].ClusterSettings().SV,
	}

	sink := makeFileSSTSink(sinkConf, store)

	getKeys := func(prefix string, n int) []byte {
		var b bytes.Buffer
		sst := storage.MakeBackupSSTWriter(ctx, nil, &b)
		for i := 0; i < n; i++ {
			require.NoError(t, sst.PutUnversioned([]byte(fmt.Sprintf("%s%08d", prefix, i)), nil))
		}
		sst.Close()
		return b.Bytes()
	}

	sp1 := exportedSpan{
		metadata: backuppb.BackupManifest_File{
			Span: roachpb.Span{
				Key:    []byte("a"),
				EndKey: []byte("b"),
			},
			Path: "1.sst",
			EntryCounts: roachpb.RowCount{
				DataSize: 100,
				Rows:     1,
			},
			StartTime:  hlc.Timestamp{},
			EndTime:    hlc.Timestamp{},
			LocalityKV: "",
		},
		dataSST:        getKeys("a", 100),
		revStart:       hlc.Timestamp{},
		completedSpans: 1,
		atKeyBoundary:  false,
	}

	sp2 := exportedSpan{
		metadata: backuppb.BackupManifest_File{
			Span: roachpb.Span{
				Key:    []byte("b"),
				EndKey: []byte("b"),
			},
			Path: "2.sst",
			EntryCounts: roachpb.RowCount{
				DataSize: 100,
				Rows:     1,
			},
			StartTime:  hlc.Timestamp{},
			EndTime:    hlc.Timestamp{},
			LocalityKV: "",
		},
		dataSST:        getKeys("b", 1),
		revStart:       hlc.Timestamp{},
		completedSpans: 1,
		atKeyBoundary:  false,
	}

	sp3 := exportedSpan{
		metadata: backuppb.BackupManifest_File{
			Span: roachpb.Span{
				Key:    []byte("b"),
				EndKey: []byte("z"),
			},
			Path: "3.sst",
			EntryCounts: roachpb.RowCount{
				DataSize: 100,
				Rows:     1,
			},
			StartTime:  hlc.Timestamp{},
			EndTime:    hlc.Timestamp{},
			LocalityKV: "",
		},
		dataSST:        getKeys("c", 100),
		revStart:       hlc.Timestamp{},
		completedSpans: 1,
		atKeyBoundary:  false,
	}

	require.NoError(t, sink.write(ctx, sp1))
	require.NoError(t, sink.write(ctx, sp3))
	require.NoError(t, sink.write(ctx, sp2))

	close(progCh)

	store.List(ctx, "", "", func(s string) error {
		fmt.Println("file", s)
		return nil
	})
	for p := range progCh {
		fmt.Println(p.String())
	}

}
