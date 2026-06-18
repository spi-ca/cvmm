package util

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type (
	// Range stores an inclusive integer interval.
	Range [2]int
	// Ranges stores ordered integer intervals for compact command argument rendering.
	Ranges []Range
)

// String formats a single value as N or an interval as N-M.
func (p Range) String() string {
	if p[0] == p[1] {
		return strconv.Itoa(p[0])
	} else {
		return fmt.Sprintf("%d-%d", p[0], p[1])
	}
}

// String joins compact range fragments with commas.
func (p Ranges) String() string {
	var items []string

	for i := 0; i < len(p); i++ {
		items = append(items, p[i].String())
	}
	return strings.Join(items, ",")
}

// ConsecutiveRanges collapses sorted integer values into compact contiguous ranges.
func ConsecutiveRanges(input []int) Ranges {
	var (
		length = 1
		list   Ranges
	)

	if len(input) == 0 {
		return nil
	}

	sorted := append([]int(nil), input...)

	sort.Ints(sorted)

	for i := 1; i <= len(sorted); i++ {
		if i == len(sorted) || sorted[i]-sorted[i-1] != 1 {
			if length == 1 {
				list = append(list, Range{sorted[i-length], sorted[i-length]})
			} else {
				list = append(list, Range{sorted[i-length], sorted[i-1]})
			}
			length = 1
		} else {
			length++
		}
	}
	return list
}
