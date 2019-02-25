/*
Copyright 2017-2019 Kaloom Inc.
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gset

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Builder is a mutable builder for GSet (Generic Set). Functions that
// mutate instances of this type are not thread-safe.
type Builder struct {
	result GSet
	done   bool
}

// GValue a value object associated with the set element name
type GValue interface{}

// GSet is a thread-safe, immutable set-like data structure for names (strings)
type GSet struct {
	elems map[string]GValue
}

// KV a key/val for GSet where the key is the set element name
type KV struct {
	Key string
	Val GValue
}

// NewBuilder returns a mutable GSet builder.
func NewBuilder() Builder {
	return Builder{
		result: GSet{
			elems: map[string]GValue{},
		},
	}
}

// Add adds the supplied elements to the result. Calling Add after calling
// Result has no effect.
func (b Builder) Add(key string, value GValue) {
	if b.done {
		return
	}
	b.result.elems[key] = value
}

// Result returns the result GSet containing all elements that were
// previously added to this builder. Subsequent calls to Add have no effect.
func (b Builder) Result() GSet {
	b.done = true
	return b.result
}

// NewGSet returns a new GSet containing the supplied elements.
func NewGSet(elems ...KV) GSet {
	b := NewBuilder()
	for _, kv := range elems {
		b.Add(kv.Key, kv.Val)
	}
	return b.Result()
}

// Size returns the number of elements in this set.
func (s GSet) Size() int {
	return len(s.elems)
}

// IsEmpty returns true if there are zero elements in this set.
func (s GSet) IsEmpty() bool {
	return s.Size() == 0
}

// Contains returns true if the supplied element is present in this set.
func (s GSet) Contains(net string) bool {
	_, found := s.elems[net]
	return found
}

// Equals returns true if the supplied set contains exactly the same elements
// as this set (s IsSubsetOf s2 and s2 IsSubsetOf s).
func (s GSet) Equals(s2 GSet) bool {
	return reflect.DeepEqual(s.elems, s2.elems)
}

// Filter returns a new G set that contains all of the elements from this
// set that match the supplied predicate, without mutating the source set.
func (s GSet) Filter(predicate func(string) bool) GSet {
	b := NewBuilder()
	for net := range s.elems {
		if predicate(net) {
			b.Add(net, s.elems[net])
		}
	}
	return b.Result()
}

// FilterNot returns a new G set that contains all of the elements from this
// set that do not match the supplied predicate, without mutating the source
// set.
func (s GSet) FilterNot(predicate func(string) bool) GSet {
	b := NewBuilder()
	for net := range s.elems {
		if !predicate(net) {
			b.Add(net, s.elems[net])
		}
	}
	return b.Result()
}

// IsSubsetOf returns true if the supplied set contains all the elements
func (s GSet) IsSubsetOf(s2 GSet) bool {
	result := true
	for net := range s.elems {
		if !s2.Contains(net) {
			result = false
			break
		}
	}
	return result
}

// Union returns a new G set that contains all of the elements from this
// set and all of the elements from the supplied set, without mutating
// either source set.
func (s GSet) Union(s2 GSet) GSet {
	b := NewBuilder()
	for net := range s.elems {
		b.Add(net, s.elems[net])
	}
	for net := range s2.elems {
		b.Add(net, s2.elems[net])
	}
	return b.Result()
}

// Intersection returns a new G set that contains all of the elements
// that are present in both this set and the supplied set, without mutating
// either source set.
func (s GSet) Intersection(s2 GSet) GSet {
	return s.Filter(func(net string) bool { return s2.Contains(net) })
}

// Difference returns a new G set that contains all of the elements that
// are present in this set and not the supplied set, without mutating either
// source set.
func (s GSet) Difference(s2 GSet) GSet {
	return s.FilterNot(func(net string) bool { return s2.Contains(net) })
}

// ToSlice returns a slice of KV that contains all elements from
// this set.
func (s GSet) ToSlice() KVSlice {
	result := KVSlice{}
	for net := range s.elems {
		result = append(result, KV{net, s.elems[net]})
	}
	sort.Sort(result)
	return result
}

// String returns a new string representation of the elements in this G set
func (s GSet) String() string {
	if s.IsEmpty() {
		return ""
	}

	// construct string from elems
	var result bytes.Buffer
	for _, e := range s.ToSlice() {
		result.WriteString("{")
		result.WriteString(e.Key)
		result.WriteString(",")
		result.WriteString(fmt.Sprintf("%v", e.Val))
		result.WriteString("}")
		result.WriteString(",")
	}
	return strings.TrimRight(result.String(), ",")
}

// Clone returns a copy of this G set.
func (s GSet) Clone() GSet {
	b := NewBuilder()
	for elem := range s.elems {
		b.Add(elem, s.elems[elem])
	}
	return b.Result()
}

// KVSlice a slice of KV
type KVSlice []KV

// Len implements KVSlice's Len method for the Sort interface
func (kv KVSlice) Len() int {
	return len(kv)
}

// Swap implements KVSlice's Swap method for the Sort interface
func (kv KVSlice) Swap(i, j int) {
	kv[i].Key, kv[j].Key = kv[j].Key, kv[i].Key
	kv[i].Val, kv[j].Val = kv[j].Val, kv[i].Val
}

// Less implements KVSlice's Less method for the Sort interface
func (kv KVSlice) Less(i, j int) bool {
	return kv[i].Key < kv[j].Key
}
