package backupccl

import (
	"bytes"
	"context"
	"github.com/cockroachdb/cockroach/pkg/ccl/storageccl"
	"github.com/cockroachdb/cockroach/pkg/cloud"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/stats"
	"github.com/cockroachdb/cockroach/pkg/storage"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
)

type SpanIterator struct {
	backing bytesIter
	filter  func(key storage.MVCCKey) bool
}

func (si *SpanIterator) Next() (bool, error) {
	ok, err := si.backing.next()
	for ; ok; ok, err = si.backing.next() {
		key, _ := si.backing.cur()

		if si.filter == nil || si.filter(key) {
			break
		}
	}

	return ok, err
}

func (si *SpanIterator) Cur() (roachpb.Span, error) {
	k, _ := si.backing.cur()

	var span roachpb.Span
	span, err := decodeSpanSSTKey(k.Key)
	if err != nil {
		return roachpb.Span{}, err
	}

	return span, nil
}

func (si *SpanIterator) Close() {
	si.backing.close()
}

func (si *SpanIterator) ContainsKey(key roachpb.Key) (bool, error) {
	ok, err := si.Next()
	for ; ok; ok, err = si.Next() {
		span, err := si.Cur()
		if err != nil {
			return false, err
		}

		if span.ContainsKey(key) {
			return true, nil
		}
	}

	return false, err
}

type FileIterator struct {
	backing bytesIter
}

func (fi *FileIterator) Next() (bool, error) {
	return fi.backing.next()
}

func (fi *FileIterator) Cur() (BackupManifest_File, error) {
	_, v := fi.backing.cur()

	var file BackupManifest_File
	err := protoutil.Unmarshal(v, &file)
	if err != nil {
		return BackupManifest_File{}, err
	}

	return file, nil
}

type DescIterator struct {
	backing bytesIter
}

func (di *DescIterator) Next() (bool, error) {
	ok, err := di.backing.next()
	for ; ok; ok, err = di.backing.next() {
		_, v := di.backing.cur()

		var desc descpb.Descriptor
		err := protoutil.Unmarshal(v, &desc)
		if err != nil {
			return false, err
		}

		tbl, db, typ, sc := descpb.FromDescriptor(&desc)
		if tbl != nil || db != nil || typ != nil || sc != nil {
			break
		}
	}

	return ok, err
}

func (di *DescIterator) Cur() (descpb.Descriptor, error) {
	_, v := di.backing.cur()

	var desc descpb.Descriptor
	err := protoutil.Unmarshal(v, &desc)

	if err != nil {
		return descpb.Descriptor{}, err
	}

	return desc, nil
}

type TenantIterator struct {
	backing bytesIter
}

func (ti *TenantIterator) Next() (bool, error) {
	return ti.backing.next()
}

func (ti *TenantIterator) Cur() (descpb.TenantInfoWithUsage, error) {
	_, v := ti.backing.cur()

	var tenant descpb.TenantInfoWithUsage
	err := protoutil.Unmarshal(v, &tenant)

	if err != nil {
		return descpb.TenantInfoWithUsage{}, err
	}

	return tenant, nil
}

func (ti *TenantIterator) Close() {
	ti.backing.close()
}

type DescriptorRevisionIterator struct {
	backing bytesIter
}

func (dri *DescriptorRevisionIterator) Next() (bool, error) {
	return dri.backing.next()
}

func (dri *DescriptorRevisionIterator) Cur() (BackupManifest_DescriptorRevision, error) {
	k, v := dri.backing.cur()

	var desc *descpb.Descriptor
	if len(v) > 0 {
		desc = &descpb.Descriptor{}
		err := protoutil.Unmarshal(v, desc)
		if err != nil {
			return BackupManifest_DescriptorRevision{}, err
		}
	}

	id, err := decodeDescSSTKey(k.Key)
	if err != nil {
		return BackupManifest_DescriptorRevision{}, err
	}

	rev := BackupManifest_DescriptorRevision{
		Desc: desc,
		ID:   id,
		Time: k.Timestamp,
	}

	return rev, nil
}

func (dri *DescriptorRevisionIterator) Close() {
	dri.backing.close()
}

func Next() {
	k := roachpb.Key{}

	k.PrefixEnd()
}

type StatsIterator struct {
	backing bytesIter
}

func (si *StatsIterator) Next() (bool, error) {
	return si.backing.next()
}

func (si *StatsIterator) Cur() (stats.TableStatisticProto, error) {
	_, v := si.backing.cur()

	var s stats.TableStatisticProto
	err := protoutil.Unmarshal(v, &s)
	if err != nil {
		return stats.TableStatisticProto{}, err
	}

	return s, nil
}

func (b *BackupManifestV2) SpanIter(ctx context.Context) SpanIterator {
	backing, err := makeBytesIter(ctx, b.store, b.sstName, []byte(sstSpansPrefix), b.enc, true)
	if err != nil {
		// TODO: return this
		panic(err)
	}

	return SpanIterator{
		backing: backing,
	}
}

func (b *BackupManifestV2) IntroducedSpanIter(ctx context.Context) SpanIterator {
	backing, err := makeBytesIter(ctx, b.store, b.sstName, []byte(sstSpansPrefix), b.enc, false)
	if err != nil {
		// TODO: return this
		panic(err)
	}

	return SpanIterator{
		backing: backing,
		filter: func(key storage.MVCCKey) bool {
			return key.Timestamp == hlc.Timestamp{}
		},
	}
}

func (b *BackupManifestV2) FileIter(ctx context.Context) FileIterator {
	backing, err := makeBytesIter(ctx, b.store, b.sstName, []byte(sstFilesPrefix), b.enc, false)
	if err != nil {
		// TODO: return this
		panic(err)
	}

	return FileIterator{
		backing: backing,
	}
}

func (b *BackupManifestV2) DescIter(ctx context.Context) DescIterator {
	backing, err := makeBytesIter(ctx, b.store, b.sstName, []byte(sstDescsPrefix), b.enc, true)
	if err != nil {
		// TODO: return this
		panic(err)
	}

	return DescIterator{
		backing: backing,
	}
}

func (b *BackupManifestV2) TenantIter(ctx context.Context) TenantIterator {
	backing, err := makeBytesIter(ctx, b.store, b.sstName, []byte(sstTenantsPrefix), b.enc, false)
	if err != nil {
		// TODO: return this
		panic(err)
	}

	return TenantIterator{
		backing: backing,
	}
}

func (b *BackupManifestV2) DescriptorChangesIter(ctx context.Context) DescriptorRevisionIterator {
	backing, err := makeBytesIter(ctx, b.store, b.sstName, []byte(sstDescsPrefix), b.enc, false)
	if err != nil {
		// TODO: return this
		panic(err)
	}

	return DescriptorRevisionIterator{
		backing: backing,
	}
}

func (b *BackupManifestV2) StatsIter(ctx context.Context) StatsIterator {
	backing, err := makeBytesIter(ctx, b.store, b.sstName, []byte(sstStatsPrefix), b.enc, false)
	if err != nil {
		// TODO: return this
		panic(err)
	}

	return StatsIterator{
		backing: backing,
	}
}

type bytesIter struct {
	ctx         context.Context
	Prefix      []byte
	Iter        storage.SimpleMVCCIterator
	key         storage.MVCCKey
	value       []byte
	useMVCCNext bool
}

func makeBytesIter(
	ctx context.Context,
	store cloud.ExternalStorage,
	path string,
	prefix []byte,
	encOpts *roachpb.FileEncryptionOptions,
	useMVCCNext bool,
) (bytesIter, error) {
	iter, err := storageccl.ExternalSSTReader(ctx, store, path, encOpts)
	if err != nil {
		return bytesIter{}, err
	}

	iter.SeekGE(storage.MakeMVCCMetadataKey(prefix))
	return bytesIter{
		Prefix:      prefix,
		Iter:        iter,
		useMVCCNext: useMVCCNext,
	}, nil
}

func (bi *bytesIter) next() (bool, error) {
	valid, err := bi.Iter.Valid()
	if err != nil || !valid || !bytes.HasPrefix(bi.Iter.UnsafeKey().Key, bi.Prefix) {
		bi.close()
		return false, err
	}

	bi.key.Key = make([]byte, 0, 1) // bi.key.Key[:0]
	bi.value = make([]byte, 0, 1)   //bi.value[:0]

	bi.key.Key = append(bi.key.Key, bi.Iter.UnsafeKey().Key...)
	bi.key.Timestamp = bi.Iter.UnsafeKey().Timestamp
	bi.value = append(bi.value, bi.Iter.UnsafeValue()...)

	if bi.useMVCCNext {
		bi.Iter.NextKey()
	} else {
		bi.Iter.Next()
	}
	return true, nil
}

func (bi *bytesIter) cur() (storage.MVCCKey, []byte) {
	return bi.key, bi.value
}

func (bi *bytesIter) close() {
	if bi.Iter != nil {
		bi.Iter.Close()
		bi.Iter = nil
	}
}
