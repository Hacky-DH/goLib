package priorityqueue

import (
	"container/heap"
)

// An Item is something we manage in a priority queue.
type Item struct {
	value    string // The value of the item; arbitrary.
	priority int    // The priority of the item in the queue.
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
}

// A PriorityQueue implements heap.Interface and holds Items.
type PriorityQueue []*Item

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	return pq[i].priority > pq[j].priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	//old[n-1] = nil
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

//UpdatePriorityQueue update or add an item to PQ
//priority can be positive or negitive
func (pq *PriorityQueue) UpdatePriorityQueue(value string, priority int) bool {
	var found *Item
	for _, item := range *pq {
		if item.value == value {
			found = item
			break
		}
	}
	if found != nil {
		found.priority += priority
		heap.Fix(pq, found.index)
		return false
	} else {
		item := &Item{
			value:    value,
			priority: priority,
			index:    len(*pq),
		}
		heap.Push(pq, item)
	}
	return true
}

func (pq *PriorityQueue) Take() *Item {
	if pq.Len() < 1 {
		return nil
	}
	return heap.Pop(pq).(*Item)
}

//InitPriorityQueue init all priority is 0
func InitPriorityQueue(items []string) *PriorityQueue {
	pq := make(PriorityQueue, len(items))
	for index, value := range items {
		pq[index] = &Item{
			value:    value,
			priority: 0,
			index:    index,
		}
	}
	heap.Init(&pq)
	return &pq
}

func InitPriorityQueueMap(items map[string]int) *PriorityQueue {
	pq := make(PriorityQueue, len(items))
	i := 0
	for value, priority := range items {
		pq[i] = &Item{
			value:    value,
			priority: priority,
			index:    i,
		}
		i++
	}
	heap.Init(&pq)
	return &pq
}

// update modifies the priority and value of an Item in the queue.
func (pq *PriorityQueue) update(item *Item, value string, priority int) {
	item.value = value
	item.priority = priority
	heap.Fix(pq, item.index)
}
