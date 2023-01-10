// Copyright 2016 The Cockroach Authors.
//
// Licensed as a CockroachDB Enterprise file under the Cockroach Community
// License (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     https://github.com/cockroachdb/cockroach/blob/master/licenses/CCL.txt

package backupccl

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/ccl/backupccl/backupinfo"
	"github.com/cockroachdb/cockroach/pkg/ccl/utilccl/sampledataccl"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
	"github.com/cockroachdb/cockroach/pkg/workload/bank"
	"github.com/stretchr/testify/require"
)

func BenchmarkDatabaseBackup(b *testing.B) {
	// NB: This benchmark takes liberties in how b.N is used compared to the go
	// documentation's description. We're getting useful information out of it,
	// but this is not a pattern to cargo-cult.

	_, sqlDB, dir, cleanupFn := backupRestoreTestSetup(b, multiNode,
		0 /* numAccounts */, InitManualReplication)
	defer cleanupFn()
	sqlDB.Exec(b, `DROP TABLE data.bank`)

	bankData := bank.FromRows(b.N).Tables()[0]
	loadURI := "nodelocal://0/load"
	if _, err := sampledataccl.ToBackup(b, bankData, dir, "load"); err != nil {
		b.Fatalf("%+v", err)
	}
	sqlDB.Exec(b, fmt.Sprintf(`RESTORE data.* FROM '%s'`, loadURI))

	// TODO(dan): Ideally, this would split and rebalance the ranges in a more
	// controlled way. A previous version of this code did it manually with
	// `SPLIT AT` and TestCluster's TransferRangeLease, but it seemed to still
	// be doing work after returning, which threw off the timing and the results
	// of the benchmark. DistSQL is working on improving this infrastructure, so
	// use what they build.

	b.ResetTimer()
	var unused string
	var dataSize int64
	sqlDB.QueryRow(b, fmt.Sprintf(`BACKUP DATABASE data TO '%s'`, localFoo)).Scan(
		&unused, &unused, &unused, &unused, &unused, &dataSize,
	)
	b.StopTimer()
	b.SetBytes(dataSize / int64(b.N))
}

func BenchmarkDatabaseRestore(b *testing.B) {
	// NB: This benchmark takes liberties in how b.N is used compared to the go
	// documentation's description. We're getting useful information out of it,
	// but this is not a pattern to cargo-cult.

	_, sqlDB, dir, cleanup := backupRestoreTestSetup(b, multiNode,
		0 /* numAccounts*/, InitManualReplication)
	defer cleanup()
	sqlDB.Exec(b, `DROP TABLE data.bank`)

	bankData := bank.FromRows(b.N).Tables()[0]
	if _, err := sampledataccl.ToBackup(b, bankData, dir, "foo"); err != nil {
		b.Fatalf("%+v", err)
	}

	b.ResetTimer()
	sqlDB.Exec(b, `RESTORE data.* FROM 'nodelocal://0/foo'`)
	b.StopTimer()
}

func BenchmarkEmptyIncrementalBackup(b *testing.B) {
	const numStatements = 100000

	_, sqlDB, dir, cleanupFn := backupRestoreTestSetup(b, multiNode,
		0 /* numAccounts */, InitManualReplication)
	defer cleanupFn()

	restoreURI := localFoo + "/restore"
	fullURI := localFoo + "/full"

	bankData := bank.FromRows(numStatements).Tables()[0]
	_, err := sampledataccl.ToBackup(b, bankData, dir, "foo/restore")
	if err != nil {
		b.Fatalf("%+v", err)
	}
	sqlDB.Exec(b, `DROP TABLE data.bank`)
	sqlDB.Exec(b, `RESTORE data.* FROM $1`, restoreURI)

	var unused string
	var dataSize int64
	sqlDB.QueryRow(b, `BACKUP DATABASE data TO $1`, fullURI).Scan(
		&unused, &unused, &unused, &unused, &unused, &dataSize,
	)

	// We intentionally don't write anything to the database between the full and
	// incremental backup.

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		incrementalDir := localFoo + fmt.Sprintf("/incremental%d", i)
		sqlDB.Exec(b, `BACKUP DATABASE data TO $1 INCREMENTAL FROM $2`, incrementalDir, fullURI)
	}
	b.StopTimer()

	// We report the number of bytes that incremental backup was able to
	// *skip*--i.e., the number of bytes in the full backup.
	b.SetBytes(int64(b.N) * dataSize)
}

func BenchmarkDatabaseFullBackup(b *testing.B) {
	const numStatements = 100000

	_, sqlDB, dir, cleanupFn := backupRestoreTestSetup(b, multiNode,
		0 /* numAccounts */, InitManualReplication)
	defer cleanupFn()

	restoreURI := localFoo + "/restore"
	fullURI := localFoo + "/full"

	bankData := bank.FromRows(numStatements).Tables()[0]
	_, err := sampledataccl.ToBackup(b, bankData, dir, "foo/restore")
	if err != nil {
		b.Fatalf("%+v", err)
	}
	sqlDB.Exec(b, `DROP TABLE data.bank`)
	sqlDB.Exec(b, `RESTORE data.* FROM $1`, restoreURI)

	var unused string
	var dataSize int64
	sqlDB.QueryRow(b, `BACKUP DATABASE data TO $1`, fullURI).Scan(
		&unused, &unused, &unused, &unused, &unused, &dataSize,
	)

	// We intentionally don't write anything to the database between the full and
	// incremental backup.

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backupDir := localFoo + fmt.Sprintf("/backup%d", i)
		sqlDB.Exec(b, `BACKUP DATABASE data TO $1`, backupDir)
	}
	b.StopTimer()

	// We report the number of bytes that incremental backup was able to
	// *skip*--i.e., the number of bytes in the full backup.
	b.SetBytes(int64(b.N) * dataSize)
}

func BenchmarkBackupMetadataReading(b *testing.B) {
	tc, _, _, cleanupFn := backupRestoreTestSetup(b, multiNode,
		0 /* numAccounts */, InitManualReplication)
	defer cleanupFn()

	// Overwrite the stats file with some invalid data.
	ctx := context.Background()
	execCfg := tc.Server(0).ExecutorConfig().(sql.ExecutorConfig)
	//store, err := execCfg.DistSQLSrv.ExternalStorageFromURI(ctx, dest, username.RootUserName()

	uri := "gs://rui-backup-test/tpce/23.1/2M/1/2022/12/05-204258.97?AUTH=implicit"
	b.ResetTimer()
	manifest, mem, err := backupinfo.ReadBackupManifestFromURI(ctx, nil, uri, username.RootUserName(), execCfg.DistSQLSrv.ExternalStorageFromURI, nil, &testKMSEnv{})
	bytes1, err := protoutil.Marshal(&manifest)
	require.NoError(b, err)
	manifest.Spans = nil
	manifest.Files = nil
	manifest.Descriptors = nil
	manifest.DescriptorChanges = nil

	bytes, err := protoutil.Marshal(&manifest)
	require.NoError(b, err)
	fmt.Printf("manifest size before=%d,%d after=%d\n", mem, len(bytes1), len(bytes))

	//for i := 0; i < b.N; i++ {
	//	_, err = backupinfo.ReadBackupMetadataFromURI(ctx, uri, username.RootUserName(), execCfg.DistSQLSrv.ExternalStorageFromURI, nil,
	//		&testKMSEnv{})
	//}
	//require.NoError(b, err)

}
