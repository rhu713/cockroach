// Copyright 2022 The Cockroach Authors.
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
	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/blobs"
	"github.com/cockroachdb/cockroach/pkg/ccl/storageccl"
	"github.com/cockroachdb/cockroach/pkg/security"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	storage2 "github.com/cockroachdb/cockroach/pkg/storage"
	"github.com/cockroachdb/cockroach/pkg/storage/cloud"
	"github.com/cockroachdb/cockroach/pkg/storage/cloudimpl"
	"testing"
)

func TestReadCorruptSST(t *testing.T) {
	dir := "s3://rui-crl/corrupt-restore?AUTH=implicit"
	file := "775739825891966995.sst"
	ctx := context.Background()
	user := security.RootUserName()

	storage, err := makeS3Storage(ctx, dir, user)
	if err != nil {
		t.Fatal(err)
	}

	iter, err := storageccl.ExternalSSTReader(ctx, storage, file, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer iter.Close()

	for iter.SeekGE(storage2.MVCCKey{}); ; {
		ok, err := iter.Valid()
		if err != nil {
			t.Fatal(err)
		}

		if !ok {
			break
		}

		keyValue := storage2.MVCCKeyValue{}
		keyValue.Key.Key = append(keyValue.Key.Key, iter.UnsafeKey().Key...)
		keyValue.Key.Timestamp = iter.UnsafeKey().Timestamp
		keyValue.Value = append(keyValue.Value, iter.UnsafeValue()...)
		fmt.Printf("kv=%v\n", keyValue)

		iter.NextKey()
	}
	fmt.Printf("PASSED")
}

func makeS3Storage(
	ctx context.Context, uri string, user security.SQLUsername,
) (cloud.ExternalStorage, error) {
	testSettings := cluster.MakeTestingClusterSettings()
	conf, err := cloudimpl.ExternalStorageConfFromURI(uri, user)
	if err != nil {
		return nil, err
	}

	// Setup a sink for the given args.
	clientFactory := blobs.TestBlobServiceClient(testSettings.ExternalIODir)
	s, err := cloudimpl.MakeExternalStorage(ctx, conf, base.ExternalIODirConfig{}, testSettings,
		clientFactory, nil, nil)
	if err != nil {
		return nil, err
	}

	return s, nil
}
