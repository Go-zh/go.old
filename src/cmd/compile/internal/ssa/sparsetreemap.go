// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

import "fmt"

// A SparseTreeMap encodes a subset of nodes within a tree
// used for sparse-ancestor queries.
//
// Combined with a SparseTreeHelper, this supports an Insert
// to add a tree node to the set and a Find operation to locate
// the nearest tree ancestor of a given node such that the
// ancestor is also in the set.
//
// Given a set of blocks {B1, B2, B3} within the dominator tree, established by
// stm.Insert()ing B1, B2, B3, etc, a query at block B
// (performed with stm.Find(stm, B, adjust, helper))
// will return the member of the set that is the nearest strict
// ancestor of B within the dominator tree, or nil if none exists.
// The expected complexity of this operation is the log of the size
// the set, given certain assumptions about sparsity (the log complexity
// could be guaranteed with additional data structures whose constant-
// factor overhead has not yet been justified.)
//
// The adjust parameter allows positioning of the insertion
// and lookup points within a block -- one of
// AdjustBefore, AdjustWithin, AdjustAfter,
// where lookups at AdjustWithin can find insertions at
// AdjustBefore in the same block, and lookups at AdjustAfter
// can find insertions at either AdjustBefore or AdjustWithin
// in the same block.  (Note that this assumes a gappy numbering
// such that exit number or exit number is separated from its
// nearest neighbor by at least 3).
//
// The Sparse Tree lookup algorithm is described by
// Paul F. Dietz. Maintaining order in a linked list. In
// Proceedings of the Fourteenth Annual ACM Symposium on
// Theory of Computing, pages 122–127, May 1982.
// and by
// Ben Wegbreit. Faster retrieval from context trees.
// Communications of the ACM, 19(9):526–529, September 1976.
type SparseTreeMap RBTint32

// A SparseTreeHelper contains indexing and allocation data
// structures common to a collection of SparseTreeMaps, as well
// as exposing some useful control-flow-related data to other
// packages, such as gc.
type SparseTreeHelper struct {
	Sdom   []SparseTreeNode // indexed by block.ID
	Po     []*Block         // exported data
	Dom    []*Block         // exported data
	Ponums []int32          // exported data
}

// NewSparseTreeHelper returns a SparseTreeHelper for use
// in the gc package, for example in phi-function placement.
func NewSparseTreeHelper(f *Func) *SparseTreeHelper {
	dom := dominators(f)
	ponums := make([]int32, f.NumBlocks())
	po := postorderWithNumbering(f, ponums)
	return makeSparseTreeHelper(newSparseTree(f, dom), dom, po, ponums)
}

func (h *SparseTreeHelper) NewTree() *SparseTreeMap {
	return &SparseTreeMap{}
}

func makeSparseTreeHelper(sdom SparseTree, dom, po []*Block, ponums []int32) *SparseTreeHelper {
	helper := &SparseTreeHelper{Sdom: []SparseTreeNode(sdom),
		Dom:    dom,
		Po:     po,
		Ponums: ponums,
	}
	return helper
}

// A sparseTreeMapEntry contains the data stored in a binary search
// data structure indexed by (dominator tree walk) entry and exit numbers.
// Each entry is added twice, once keyed by entry-1/entry/entry+1 and
// once keyed by exit+1/exit/exit-1. (there are three choices of paired indices, not 9, and they properly nest)
type sparseTreeMapEntry struct {
	index *SparseTreeNode
	block *Block // TODO: store this in a separate index.
	data  interface{}
}

// Insert creates a definition within b with data x.
// adjust indicates where in the block should be inserted:
// AdjustBefore means defined at a phi function (visible Within or After in the same block)
// AdjustWithin means defined within the block (visible After in the same block)
// AdjustAfter means after the block (visible within child blocks)
func (m *SparseTreeMap) Insert(b *Block, adjust int32, x interface{}, helper *SparseTreeHelper) {
	rbtree := (*RBTint32)(m)
	blockIndex := &helper.Sdom[b.ID]
	if blockIndex.entry == 0 {
		// assert unreachable
		return
	}
	entry := &sparseTreeMapEntry{index: blockIndex, data: x}
	right := blockIndex.exit - adjust
	_ = rbtree.Insert(right, entry)

	left := blockIndex.entry + adjust
	_ = rbtree.Insert(left, entry)
}

// Find returns the definition visible from block b, or nil if none can be found.
// Adjust indicates where the block should be searched.
// AdjustBefore searches before the phi functions of b.
// AdjustWithin searches starting at the phi functions of b.
// AdjustAfter searches starting at the exit from the block, including normal within-block definitions.
//
// Note that Finds are properly nested with Inserts:
// m.Insert(b, a) followed by m.Find(b, a) will not return the result of the insert,
// but m.Insert(b, AdjustBefore) followed by m.Find(b, AdjustWithin) will.
//
// Another way to think of this is that Find searches for inputs, Insert defines outputs.
func (m *SparseTreeMap) Find(b *Block, adjust int32, helper *SparseTreeHelper) interface{} {
	rbtree := (*RBTint32)(m)
	if rbtree == nil {
		return nil
	}
	blockIndex := &helper.Sdom[b.ID]
	_, v := rbtree.Glb(blockIndex.entry + adjust)
	for v != nil {
		otherEntry := v.(*sparseTreeMapEntry)
		otherIndex := otherEntry.index
		// Two cases -- either otherIndex brackets blockIndex,
		// or it doesn't.
		//
		// Note that if otherIndex and blockIndex are
		// the same block, then the glb test only passed
		// because the definition is "before",
		// i.e., k == blockIndex.entry-1
		// allowing equality is okay on the blocks check.
		if otherIndex.exit >= blockIndex.exit {
			// bracketed.
			return otherEntry.data
		}
		// In the not-bracketed case, we could memoize the results of
		// walking up the tree, but for now we won't.
		// Memoize plan is to take the gap (inclusive)
		// from otherIndex.exit+1 to blockIndex.entry-1
		// and insert it into this or a second tree.
		// Said tree would then need adjusting whenever
		// an insertion occurred.

		// Expectation is that per-variable tree is sparse,
		// therefore probe siblings instead of climbing up.
		// Note that each sibling encountered in this walk
		// to find a defining ancestor shares that ancestor
		// because the walk skips over the interior -- each
		// Glb will be an exit, and the iteration is to the
		// Glb of the entry.
		_, v = rbtree.Glb(otherIndex.entry - 1)
	}
	return nil // nothing found
}

func (m *SparseTreeMap) String() string {
	tree := (*RBTint32)(m)
	return tree.String()
}

func (e *sparseTreeMapEntry) String() string {
	return fmt.Sprintf("index=%v, data=%v", e.index, e.data)
}
