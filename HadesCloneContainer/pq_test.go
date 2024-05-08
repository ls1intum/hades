package main

import (
	"container/heap"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPush(t *testing.T) {
	pq := make(PriorityQueue, 0)
	heap.Init(&pq)

	// Add some items to the priority queue
	item1 := &Item{Repository: Repository{}, order: 1}
	item2 := &Item{Repository: Repository{}, order: 2}
	item3 := &Item{Repository: Repository{}, order: 3}
	heap.Push(&pq, item1)
	heap.Push(&pq, item2)
	heap.Push(&pq, item3)

	assert.Equal(t, 3, pq.Len()) // Assert the length of the priority queue after adding items
}

func TestPop(t *testing.T) {
	pq := make(PriorityQueue, 0)
	heap.Init(&pq)

	// Add some items to the priority queue
	item1 := &Item{Repository: Repository{}, order: 1}
	item2 := &Item{Repository: Repository{}, order: 2}
	item3 := &Item{Repository: Repository{}, order: 3}
	heap.Push(&pq, item1)
	heap.Push(&pq, item2)
	heap.Push(&pq, item3)

	// Pop an item from the priority queue
	poppedItem := heap.Pop(&pq).(*Item)

	// Assert the popped item is the expected item
	assert.Equal(t, item1, poppedItem)
	assert.Equal(t, 2, pq.Len()) // Assert the length of the priority queue after popping

	// Assert the order of the remaining items in the priority queue
	remainingItem1 := pq[0]
	remainingItem2 := pq[1]
	assert.Equal(t, 2, remainingItem1.order)
	assert.Equal(t, 3, remainingItem2.order)
}
