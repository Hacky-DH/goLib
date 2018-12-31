package detectutil

import (
    "testing"
)

func TestPq(t *testing.T) {
    // Some items and their priorities.
    items := map[string]int{
        "banana": 3, "apple": 2, "pear": 4,
    }
    pq := InitPriorityQueueMap(items)
    
    // Insert a new item and then modify its priority.
    pq.UpdatePriorityQueue("orange", 5)

    // Take the items out; they arrive in decreasing priority order.
    for i:=5; i>1; i-- {
        item := pq.Take()
        if item.priority != i {
            t.Error("priority error")
        }
    }
}