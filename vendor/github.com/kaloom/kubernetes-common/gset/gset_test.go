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
	"reflect"
	"testing"
)

func TestGSetBuilder(t *testing.T) {
	b := NewBuilder()
	elems := []string{"net1", "net2", "net3", "net4", "net5"}
	for _, elem := range elems {
		b.Add(elem, "")
	}
	result := b.Result()
	for _, elem := range elems {
		if !result.Contains(elem) {
			t.Fatalf("expected netset to contain element %s: [%v]", elem, result)
		}
	}
	if len(elems) != result.Size() {
		t.Fatalf("expected netset %s to have the same size as %v", result, elems)
	}
}

func TestGSetSize(t *testing.T) {
	testCases := []struct {
		netset   GSet
		expected int
	}{
		{NewGSet(), 0},
		{NewGSet(KV{"net5", ""}), 1},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), 5},
	}

	for _, c := range testCases {
		actual := c.netset.Size()
		if actual != c.expected {
			t.Fatalf("expected: %d, actual: %d, netset: [%v]", c.expected, actual, c.netset)
		}
	}
}

func TestGSetIsEmpty(t *testing.T) {
	testCases := []struct {
		netset   GSet
		expected bool
	}{
		{NewGSet(), true},
		{NewGSet(KV{"net5", ""}), false},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), false},
	}

	for _, c := range testCases {
		actual := c.netset.IsEmpty()
		if actual != c.expected {
			t.Fatalf("expected: %t, IsEmpty() returned: %t, netset: [%v]", c.expected, actual, c.netset)
		}
	}
}

func TestGSetContains(t *testing.T) {
	testCases := []struct {
		netset         GSet
		mustContain    []string
		mustNotContain []string
	}{
		{NewGSet(), []string{}, []string{"net1", "net2", "net3", "net4", "net5"}},
		{NewGSet(KV{"net5", ""}), []string{"net5"}, []string{"net1", "net2", "net3", "net4"}},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net4", ""}, KV{"net5", ""}), []string{"net1", "net2", "net4", "net5"}, []string{"0", "net3", "6"}},
	}

	for _, c := range testCases {
		for _, elem := range c.mustContain {
			if !c.netset.Contains(elem) {
				t.Fatalf("expected netset to contain element %s: [%v]", elem, c.netset)
			}
		}
		for _, elem := range c.mustNotContain {
			if c.netset.Contains(elem) {
				t.Fatalf("expected netset not to contain element %s: [%v]", elem, c.netset)
			}
		}
	}
}

func TestGSetEqual(t *testing.T) {
	shouldEqual := []struct {
		s1 GSet
		s2 GSet
	}{
		{NewGSet(), NewGSet()},
		{NewGSet(KV{"net5", ""}), NewGSet(KV{"net5", ""})},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},
	}

	shouldNotEqual := []struct {
		s1 GSet
		s2 GSet
	}{
		{NewGSet(), NewGSet(KV{"net5", ""})},
		{NewGSet(KV{"net5", ""}), NewGSet()},
		{NewGSet(), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet()},
		{NewGSet(KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net5", ""})},
	}

	for _, c := range shouldEqual {
		if !c.s1.Equals(c.s2) {
			t.Fatalf("expected netsets to be equal: s1: [%v], s2: [%v]", c.s1, c.s2)
		}
	}
	for _, c := range shouldNotEqual {
		if c.s1.Equals(c.s2) {
			t.Fatalf("expected netsets to not be equal: s1: [%v], s2: [%v]", c.s1, c.s2)
		}
	}
}

func TestGSetIsSubsetOf(t *testing.T) {
	shouldBeSubset := []struct {
		s1 GSet
		s2 GSet
	}{
		// A set is a subset of itself
		{NewGSet(), NewGSet()},
		{NewGSet(KV{"net5", ""}), NewGSet(KV{"net5", ""})},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},

		// Empty set is a subset of every set
		{NewGSet(), NewGSet(KV{"net5", ""})},
		{NewGSet(), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},

		{NewGSet(KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},
		{NewGSet(KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},
		{NewGSet(KV{"net2", ""}, KV{"net3", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},
	}

	shouldNotBeSubset := []struct {
		s1 GSet
		s2 GSet
	}{}

	for _, c := range shouldBeSubset {
		if !c.s1.IsSubsetOf(c.s2) {
			t.Fatalf("expected s1 to be a subset of s2: s1: [%v], s2: [%v]", c.s1, c.s2)
		}
	}
	for _, c := range shouldNotBeSubset {
		if c.s1.IsSubsetOf(c.s2) {
			t.Fatalf("expected s1 to not be a subset of s2: s1: [%v], s2: [%v]", c.s1, c.s2)
		}
	}
}

func TestGSetUnion(t *testing.T) {
	testCases := []struct {
		s1       GSet
		s2       GSet
		expected GSet
	}{
		{NewGSet(), NewGSet(), NewGSet()},

		{NewGSet(), NewGSet(KV{"net5", ""}), NewGSet(KV{"net5", ""})},
		{NewGSet(KV{"net5", ""}), NewGSet(), NewGSet(KV{"net5", ""})},
		{NewGSet(KV{"net5", ""}), NewGSet(KV{"net5", ""}), NewGSet(KV{"net5", ""})},

		{NewGSet(), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},

		{NewGSet(KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},

		{NewGSet(KV{"net1", ""}, KV{"net2", ""}), NewGSet(KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}), NewGSet(KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},
	}

	for _, c := range testCases {
		result := c.s1.Union(c.s2)
		if !result.Equals(c.expected) {
			t.Fatalf("expected the union of s1 and s2 to be [%v] (got [%v]), s1: [%v], s2: [%v]", c.expected, result, c.s1, c.s2)
		}
	}
}

func TestGSetIntersection(t *testing.T) {
	testCases := []struct {
		s1       GSet
		s2       GSet
		expected GSet
	}{
		{NewGSet(), NewGSet(), NewGSet()},

		{NewGSet(), NewGSet(KV{"net5", ""}), NewGSet()},
		{NewGSet(KV{"net5", ""}), NewGSet(), NewGSet()},
		{NewGSet(KV{"net5", ""}), NewGSet(KV{"net5", ""}), NewGSet(KV{"net5", ""})},

		{NewGSet(), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet()},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(), NewGSet()},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},

		{NewGSet(KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net5", ""})},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net5", ""}), NewGSet(KV{"net5", ""})},

		{NewGSet(KV{"net1", ""}, KV{"net2", ""}), NewGSet(KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet()},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}), NewGSet(KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net3", ""})},
	}

	for _, c := range testCases {
		result := c.s1.Intersection(c.s2)
		if !result.Equals(c.expected) {
			t.Fatalf("expected the intersection of s1 and s2 to be [%v] (got [%v]), s1: [%v], s2: [%v]", c.expected, result, c.s1, c.s2)
		}
	}
}

func TestGSetDifference(t *testing.T) {
	testCases := []struct {
		s1       GSet
		s2       GSet
		expected GSet
	}{
		{NewGSet(), NewGSet(), NewGSet()},

		{NewGSet(), NewGSet(KV{"net5", ""}), NewGSet()},
		{NewGSet(KV{"net5", ""}), NewGSet(), NewGSet(KV{"net5", ""})},
		{NewGSet(KV{"net5", ""}), NewGSet(KV{"net5", ""}), NewGSet()},

		{NewGSet(), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet()},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""})},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet()},

		{NewGSet(KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet()},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""})},

		{NewGSet(KV{"net1", ""}, KV{"net2", ""}), NewGSet(KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""})},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}), NewGSet(KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), NewGSet(KV{"net1", ""}, KV{"net2", ""})},
	}

	for _, c := range testCases {
		result := c.s1.Difference(c.s2)
		if !result.Equals(c.expected) {
			t.Fatalf("expected the difference of s1 and s2 to be [%v] (got [%v]), s1: [%v], s2: [%v]", c.expected, result, c.s1, c.s2)
		}
	}
}

func TestGSetToSlice(t *testing.T) {
	testCases := []struct {
		set      GSet
		expected KVSlice
	}{
		{NewGSet(), KVSlice{}},
		{NewGSet(KV{"net5", ""}), KVSlice{KV{"net5", ""}}},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), KVSlice{KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}}},
	}

	for _, c := range testCases {
		result := c.set.ToSlice()
		if !reflect.DeepEqual(result, c.expected) {
			t.Fatalf("expected set as slice to be [%v] (got [%v]), s: [%v]", c.expected, result, c.set)
		}
	}
}

func TestGSetString(t *testing.T) {
	testCases := []struct {
		set      GSet
		expected string
	}{
		{NewGSet(), ""},
		{NewGSet(KV{"net5", ""}), "{net5,}"},
		{NewGSet(KV{"net1", ""}, KV{"net2", ""}, KV{"net3", ""}, KV{"net4", ""}, KV{"net5", ""}), "{net1,},{net2,},{net3,},{net4,},{net5,}"},
	}

	for _, c := range testCases {
		result := c.set.String()
		if result != c.expected {
			t.Fatalf("expected set as string to be %s (got \"%s\"), s: [%v]", c.expected, result, c.set)
		}
	}
}
