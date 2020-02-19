package utility

import (
	"fmt"
	"sync"
)

//CircularQueue circular queue structure
type CircularQueue struct {
	startPoint, endPoint int
	/*
		carryBit = false:
			size:endPoint-startPoint,
			in this case,endPoint must smaller than startPoint
		carryBit = true:
			size:endPoint-startPoint+capacity
			in this case,endPoint must bigger than startPoint

		capacity:the capacity of this array
	*/
	size, capacity int
	/*
		carryBit:
			once the endPoint change position from end of array to the starting,
			the carryBit should change from 0 to 1;
			once the startPoint change position from end of array to the starting,
			the carryBit should change from 1 to 0;
	*/
	carryBit int
	/*
		used for queue to support atom
	*/
	mutex sync.Mutex
	/*
		used for
			consumer block when queue is empty
			producer block when queue is full
	*/
	cond *sync.Cond
	/*
		when size is more than mutexThreshold,
		add item or consume item will not lock the queue
	*/
	mutexThreshold int
	queue          []interface{}
	ifAlwaysAtom   bool //if always locking queue when operating on it
}

//Init init queue of 'capacity' size
func (circularQueue *CircularQueue) Init(
	capacity, mutexThreshold int, ifAlwaysAtom bool) error {
	if !ifAlwaysAtom && mutexThreshold >= capacity {
		return fmt.Errorf("the ifAlwaysAtom is false but mutexThreshold >= capacity")
	}
	circularQueue.queue = make([]interface{}, capacity)
	circularQueue.capacity = capacity
	circularQueue.mutexThreshold = mutexThreshold
	circularQueue.ifAlwaysAtom = ifAlwaysAtom
	circularQueue.cond = sync.NewCond(&sync.Mutex{})
	return nil
}

//AddItem add the item to the queue at endPoint
func (circularQueue *CircularQueue) AddItem(item interface{}) error {
	if circularQueue.size == circularQueue.capacity {
		//@todo full, so we need wait for consuming
		return fmt.Errorf("queue full")
	}
	if circularQueue.ifAlwaysAtom ||
		circularQueue.size < circularQueue.mutexThreshold {
		circularQueue.mutex.Lock()
		defer circularQueue.mutex.Unlock()
	}

	if (circularQueue.endPoint + 1) >= circularQueue.capacity {
		if circularQueue.carryBit == 1 {
			return fmt.Errorf("endPoint + 1 >= capacity but carry bit is 1")
		}
		circularQueue.carryBit = 1
	}
	circularQueue.queue[circularQueue.endPoint] = item
	circularQueue.endPoint = (circularQueue.endPoint + 1) % circularQueue.capacity
	circularQueue.size =
		circularQueue.endPoint + circularQueue.carryBit*circularQueue.capacity -
			circularQueue.startPoint
	return nil
}

//ConsumeItem consume one item from the queue
func (circularQueue *CircularQueue) ConsumeItem() (interface{}, error) {
	if circularQueue.size == 0 {
		//@todo empty
		return nil, fmt.Errorf("queue empty")
	}
	if circularQueue.ifAlwaysAtom ||
		circularQueue.size < circularQueue.mutexThreshold {
		circularQueue.mutex.Lock()
		defer circularQueue.mutex.Unlock()
	}
	if (circularQueue.startPoint + 1) >= circularQueue.capacity {
		if circularQueue.carryBit == 0 {
			return nil, fmt.Errorf("startPoint + 1 >= capacity but carry bit is 0")
		}
		circularQueue.carryBit = 0
	}
	item := circularQueue.queue[circularQueue.startPoint]
	circularQueue.startPoint = (circularQueue.startPoint + 1) % circularQueue.capacity
	circularQueue.size =
		circularQueue.endPoint + circularQueue.carryBit*circularQueue.capacity -
			circularQueue.startPoint
	return item, nil
}
