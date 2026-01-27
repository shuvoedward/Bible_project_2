package scheduler

import "container/heap"

type PriorityQueue []*Task

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].ExecuteAt.Before(pq[j].ExecuteAt)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PriorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*Task))
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	task := old[n-1]
	*pq = old[0 : n-1]
	return task
}

func (pq *PriorityQueue) Peek() interface{} {
	if pq.Len() == 0 {
		return nil
	}
	return (*pq)[0]
}

func BuildMinHeap() *PriorityQueue {
	minHeap := &PriorityQueue{}
	heap.Init(minHeap)
	return minHeap
}
