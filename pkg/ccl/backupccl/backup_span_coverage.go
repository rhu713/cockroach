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
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/util/span"
	"github.com/cockroachdb/errors"
	"path"
)

// checkCoverage verifies that spans are covered by a given chain of backups.
// TODO: during planning, the last backup doesn't have a metadata SST
func checkCoveragePlanning(ctx context.Context, prevs []BackupManifestV2, backupManifest *BackupManifest,
	spans []roachpb.Span, introducedSpans []roachpb.Span) error {
	if len(spans) == 0 {
		return nil
	}

	frontier, err := span.MakeFrontier(spans...)
	if err != nil {
		return err
	}

	// The main loop below requires the entire frontier be caught up to the start
	// time of each step it proceeds, however a span introduced in a later backup
	// would hold back the whole frontier at 0 until it is reached, so run through
	// all layers first to unconditionally advance the introduced spans.
	for i := range prevs {
		it := prevs[i].IntroducedSpanIter(ctx)
		ok, err := it.Next()
		for ; ok; ok, err = it.Next() {
			sp, err := it.Cur()
			if err != nil {
				return err
			}
			if _, err := frontier.Forward(sp, prevs[i].StartTime); err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}
	}
	for _, sp := range introducedSpans {
		if _, err := frontier.Forward(sp, backupManifest.StartTime); err != nil {
			return err
		}
	}

	// Walk through the chain of backups in order advancing the spans each covers
	// and verify that the entire required span frontier is covered as expected.
	for i := range prevs {
		// This backup advances its covered spans _from its start time_ to its end
		// time, so before actually advance those spans in the frontier to that end
		// time, assert that it is starting at the start time, i.e. that this
		// backup does indeed pick up where the prior backup left off.
		if start, required := frontier.Frontier(), prevs[i].StartTime; start.Less(required) {
			s := frontier.PeekFrontierSpan()
			return errors.Errorf(
				"no backup covers time [%s,%s) for range [%s,%s) (or backups listed out of order)",
				start, required, s.Key, s.EndKey,
			)
		}

		// Advance every span the backup covers to its end time.
		it := prevs[i].SpanIter(ctx)
		ok, err := it.Next()
		for ; ok; ok, err = it.Next() {
			s, err := it.Cur()
			if err != nil {
				return err
			}
			if _, err := frontier.Forward(s, prevs[i].EndTime); err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}
		// Check that the backup actually covered all the required spans.
		if end, required := frontier.Frontier(), prevs[i].EndTime; end.Less(required) {
			prefix := path.Dir(prevs[i].sstName)
			manifestName := backupManifestName
			if prefix != "" {
				manifestName = path.Join(prefix, manifestName)
			}

			return errors.Errorf("expected previous backups to cover until time %v, got %v (e.g. span %v)",
				required, end, frontier.PeekFrontierSpan())
		}
	}

	{
		if start, required := frontier.Frontier(), backupManifest.StartTime; start.Less(required) {
			s := frontier.PeekFrontierSpan()
			return errors.Errorf(
				"no backup covers time [%s,%s) for range [%s,%s) (or backups listed out of order)",
				start, required, s.Key, s.EndKey,
			)
		}

		// Advance every span the backup covers to its end time.
		for _, s := range spans {
			if _, err := frontier.Forward(s, backupManifest.EndTime); err != nil {
				return err
			}
		}

		// Check that the backup actually covered all the required spans.
		if end, required := frontier.Frontier(), backupManifest.EndTime; end.Less(required) {
			return errors.Errorf("expected previous backups to cover until time %v, got %v (e.g. span %v)",
				required, end, frontier.PeekFrontierSpan())
		}
	}

	return nil
}

func checkCoverageRestore(ctx context.Context, spans []roachpb.Span, backups []BackupManifestV2) error {
	if len(spans) == 0 {
		return nil
	}

	frontier, err := span.MakeFrontier(spans...)
	if err != nil {
		return err
	}

	// The main loop below requires the entire frontier be caught up to the start
	// time of each step it proceeds, however a span introduced in a later backup
	// would hold back the whole frontier at 0 until it is reached, so run through
	// all layers first to unconditionally advance the introduced spans.
	for i := range backups {
		it := backups[i].IntroducedSpanIter(ctx)
		ok, err := it.Next()
		for ; ok; ok, err = it.Next() {
			sp, err := it.Cur()
			if err != nil {
				return err
			}
			if _, err := frontier.Forward(sp, backups[i].StartTime); err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}
	}

	// Walk through the chain of backups in order advancing the spans each covers
	// and verify that the entire required span frontier is covered as expected.
	for i := range backups {
		// This backup advances its covered spans _from its start time_ to its end
		// time, so before actually advance those spans in the frontier to that end
		// time, assert that it is starting at the start time, i.e. that this
		// backup does indeed pick up where the prior backup left off.
		if start, required := frontier.Frontier(), backups[i].StartTime; start.Less(required) {
			s := frontier.PeekFrontierSpan()
			return errors.Errorf(
				"no backup covers time [%s,%s) for range [%s,%s) (or backups listed out of order)",
				start, required, s.Key, s.EndKey,
			)
		}

		// Advance every span the backup covers to its end time.
		it := backups[i].SpanIter(ctx)
		ok, err := it.Next()
		for ; ok; ok, err = it.Next() {
			s, err := it.Cur()
			if err != nil {
				return err
			}
			if _, err := frontier.Forward(s, backups[i].EndTime); err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}

		// Check that the backup actually covered all the required spans.
		if end, required := frontier.Frontier(), backups[i].EndTime; end.Less(required) {
			return errors.Errorf("expected previous backups to cover until time %v, got %v (e.g. span %v)",
				required, end, frontier.PeekFrontierSpan())
		}
	}

	return nil
}
