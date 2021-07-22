// Copyright 2018 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package xform

import (
	"github.com/cockroachdb/cockroach/pkg/sql/opt/opt"
)

// exprState is 12 bytes of opaque storage used to store operator-specific
// fields in the memo expression.
type exprState [3]uint32

// memoExpr is a memoized representation of an expression. Strongly-typed
// specializations of memoExpr are generated by optgen for each operator (see
// expr.og.go). Each memoExpr belongs to a memo group, which contains logically
// equivalent expressions. Two expressions are considered logically equivalent
// if they both reduce to an identical normal form after normalizing
// transformations have been applied.
//
// The children of memoExpr are recursively memoized in the same way as the
// memoExpr, and are referenced by their memo group. Therefore, the memoExpr
// is the root of a forest of expressions.
type memoExpr struct {
	// op is this expression's operator type. Each operator may have additional
	// fields. To access these fields in a strongly-typed way, use the asXXX()
	// generated methods to cast the memoExpr to the more specialized
	// expression type.
	op opt.Operator

	// state stores operator-specific state. Depending upon the value of the
	// op field, this state will be interpreted in different ways.
	state exprState
}

// fingerprint uniquely identifies a memo expression by combining its operator
// type plus its operator fields. It can be used as a map key. If two
// expressions share the same fingerprint, then they are the identical
// expression. If they don't share a fingerprint, then they still may be
// logically equivalent expressions. Since a memo expression is 16 bytes and
// contains no pointers, it can function as its own fingerprint/hash.
type fingerprint memoExpr

// fingerprint returns this memo expression's unique fingerprint.
func (me memoExpr) fingerprint() fingerprint {
	return fingerprint(me)
}
