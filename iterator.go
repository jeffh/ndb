package ndb

import (
	"context"
	"sync/atomic"
)

type Iterator interface {
	Record() *Record
	Err() error
	Close()
	Next() bool
}

///////////////

type iterResult struct {
	record *Record
	err    error
}

type lazyIterator struct {
	ctx    context.Context
	cancel context.CancelFunc
	ch     chan Iterator

	ready chan iterResult

	last iterResult
}

func (it *lazyIterator) Record() *Record { return it.last.record }
func (it *lazyIterator) Err() error      { return it.last.err }
func (it *lazyIterator) Close() {
	if it.cancel != nil {
		it.cancel()
	}
}
func (it *lazyIterator) startPump() {
	it.ready = make(chan iterResult, len(it.ch))

	go func() {
		activeIterators := int32(1)
		closed := int32(0) // 0 = open, 1 = can close, 2 = already closed
		cleanup := func() {
			active := atomic.AddInt32(&activeIterators, -1)

			if active == 0 {
				c := atomic.AddInt32(&closed, 1)
				if c == 1 {
					close(it.ready)
				}
			}
		}
		for iter := range it.ch {
			atomic.AddInt32(&activeIterators, 1)

			go func(iter Iterator) {
				defer func() {
					iter.Close()
					cleanup()
				}()
				for iter.Next() {
					r := iter.Record()
					res := iterResult{record: r}
					select {
					case <-it.ctx.Done():
						return
					case it.ready <- res:
					}
				}

				if err := iter.Err(); err != nil {
					res := iterResult{err: err}
					select {
					case <-it.ctx.Done():
						return
					case it.ready <- res:
					}
				}
			}(iter)
		}
		cleanup()
	}()
}
func (it *lazyIterator) Next() bool {
	if it.ready == nil {
		it.startPump()
	}
	select {
	case <-it.ctx.Done():
		it.last.err = it.ctx.Err()
		return false
	case res, ok := <-it.ready:
		if ok {
			it.last = res
			return res.err == nil
		} else {
			it.last = iterResult{}
			return false
		}
	}
}

///////////////

type searchIterator struct {
	db         *DB
	key, value string

	ctx         context.Context
	recordIndex int

	lastErr    error
	lastRecord *Record

	locked bool
}

func makeSearchIterator(ctx context.Context, db *DB, locked bool, key, value string) *searchIterator {
	if locked {
		db.M.RLock()
	}
	return &searchIterator{
		db:     db,
		key:    key,
		value:  value,
		ctx:    ctx,
		locked: locked,
	}
}

func (it *searchIterator) Record() *Record { return it.lastRecord }
func (it *searchIterator) Err() error      { return it.lastErr }
func (it *searchIterator) Close() {
	if it.locked {
		it.db.M.RUnlock()
	}
}
func (it *searchIterator) Next() bool {
	steps := 0
	for size := len(it.db.Records); it.recordIndex < size; it.recordIndex++ {
		if steps == 5000 { // reduce impact of this check by doing it very infrequently
			steps = 0
			select {
			case <-it.ctx.Done():
				it.lastErr = it.ctx.Err()
				it.lastRecord = nil
				return false
			default:
			}
		}
		steps++

		record := &it.db.Records[it.recordIndex]
		for i, k := range record.keys {
			if k == it.key && record.values[i] == it.value {
				it.lastRecord = record
				it.lastErr = nil
				it.recordIndex++
				return true
			}
		}
	}

	return false
}

///////////////

type predicateSearchIterator struct {
	db        *DB
	predicate PredicateFunc

	ctx         context.Context
	recordIndex int

	lastErr    error
	lastRecord *Record

	locked bool
}

func makePredicateSearchIterator(ctx context.Context, db *DB, locked bool, pred PredicateFunc) *predicateSearchIterator {
	if locked {
		db.M.RLock()
	}
	return &predicateSearchIterator{
		db:        db,
		predicate: pred,
		ctx:       ctx,
		locked:    locked,
	}
}

func (it *predicateSearchIterator) Record() *Record { return it.lastRecord }
func (it *predicateSearchIterator) Err() error      { return it.lastErr }
func (it *predicateSearchIterator) Close() {
	if it.locked {
		it.db.M.RUnlock()
	}
}
func (it *predicateSearchIterator) Next() bool {
	steps := 0
	for size := len(it.db.Records); it.recordIndex < size; it.recordIndex++ {
		if steps == 5000 { // reduce impact of this check by doing it very infrequently
			steps = 0
			select {
			case <-it.ctx.Done():
				it.lastErr = it.ctx.Err()
				it.lastRecord = nil
				return false
			default:
			}
		}
		steps++

		record := &it.db.Records[it.recordIndex]
		if it.predicate(record) {
			it.lastRecord = record
			it.lastErr = nil
			it.recordIndex++
			return true
		}
	}

	return false
}
