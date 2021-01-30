package ndb

import "context"

type Iterator interface {
	Record() *Record
	Err() error
	Close()
	Next() bool
}

///////////////

type lazyIterator struct {
	ctx    context.Context
	cancel context.CancelFunc
	ch     chan Iterator

	curr Iterator
}

func (it *lazyIterator) Record() *Record {
	if it.curr != nil {
		return it.curr.Record()
	}
	return nil
}
func (it *lazyIterator) Err() error {
	if it.curr != nil {
		return it.curr.Err()
	}
	return nil
}
func (it *lazyIterator) Close() {
	if it.curr != nil {
		it.curr.Close()
	}
}
func (it *lazyIterator) Next() bool {
	for {
		var res bool
		if it.curr != nil {
			res = it.curr.Next()
			if it.Err() != nil {
				return false
			}
		}
		if !res {
			var ok bool
			it.curr, ok = <-it.ch
			if !ok {
				return false
			}
			continue
		}
		return res
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
	select {
	case <-it.ctx.Done():
		it.lastErr = it.ctx.Err()
		it.lastRecord = nil
		return false
	default:
	}

	for size := len(it.db.Records); it.recordIndex < size; it.recordIndex++ {
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
