package ndb

import (
	"bufio"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Deterministic fixture seed. Benches generate files at run time into a
// process temp dir (one file per size, cached). Nothing under testdata/
// is committed.
const benchSeed int64 = 0x6e6462 // "ndb"

const (
	benchSizeSmall  = 64 << 10  // 64 KiB
	benchSizeMedium = 8 << 20   // 8 MiB
	benchSizeLarge  = 128 << 20 // 128 MiB; gated
	benchTargetSys  = "bench-target"
	benchQuotedNote = "a b # c"
)

// benchSink retains results so the compiler cannot elide the scan.
var benchSink int

type benchSize struct {
	name       string
	target     int
	reportPeak bool
	large      bool
}

func allBenchSizes() []benchSize {
	return []benchSize{
		{name: "small", target: benchSizeSmall},
		{name: "medium", target: benchSizeMedium, reportPeak: true},
		{name: "large", target: benchSizeLarge, reportPeak: true, large: true},
	}
}

func skipGatedSize(b *testing.B, sz benchSize) {
	b.Helper()
	if !sz.large {
		return
	}
	if testing.Short() {
		b.Skip("100MB+ benches skipped under -short")
	}
	if os.Getenv("NDB_BENCH_LARGE") == "" {
		b.Skip("100MB+ benches require NDB_BENCH_LARGE=1")
	}
}

type benchDB struct {
	db      *Ndb
	size    int64
	recs    int
	sysHits int
}

var (
	fixtureMu    sync.Mutex
	fixtureCache = map[int]benchDB{}
)

// cachedBenchDB writes each size once and reuses the opened *Ndb. Go reruns
// the whole benchmark function during calibration; without a cache that
// rewrites the fixture on every round.
func cachedBenchDB(b *testing.B, sz benchSize) benchDB {
	b.Helper()
	skipGatedSize(b, sz)

	fixtureMu.Lock()
	defer fixtureMu.Unlock()
	if env, ok := fixtureCache[sz.target]; ok {
		return env
	}

	dir, err := os.MkdirTemp("", "ndb-bench-"+sz.name+"-")
	if err != nil {
		b.Fatal(err)
	}
	path := filepath.Join(dir, "bench.ndb")
	st, err := writeBenchFixture(path, sz.target)
	if err != nil {
		b.Fatal(err)
	}
	db, err := OpenOne(&LocalFileSystem{}, path)
	if err != nil {
		b.Fatal(err)
	}
	env := benchDB{db: db, size: st.size, recs: st.records, sysHits: st.sysHits}
	fixtureCache[sz.target] = env
	return env
}

type fixtureStats struct {
	records int
	sysHits int
	size    int64
}

func writeBenchFixture(path string, targetBytes int) (st fixtureStats, err error) {
	f, err := os.Create(path)
	if err != nil {
		return st, err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()

	w := bufio.NewWriterSize(f, 1<<20)
	rng := rand.New(rand.NewSource(benchSeed))
	owners := []string{"alice", "bob", "carol", "dave", "erin", "frank"}

	var buf []byte
	written := 0
	for i := 0; written < targetBytes || i < 2; i++ {
		owner := owners[rng.Intn(len(owners))]
		extra := i % 4
		buf = buf[:0]
		if i%100 == 0 {
			buf = append(buf, "# generated fixture\n"...)
		}
		// Two-line records exercise the public continuation-line parser.
		buf = append(buf, "ip=10."...)
		buf = strconv.AppendInt(buf, int64((i>>16)&0xff), 10)
		buf = append(buf, '.')
		buf = strconv.AppendInt(buf, int64((i>>8)&0xff), 10)
		buf = append(buf, '.')
		buf = strconv.AppendInt(buf, int64(i&0xff), 10)
		buf = append(buf, " sys="...)
		if i == 1 {
			buf = append(buf, benchTargetSys...)
			buf = append(buf, " dom=target.example.net"...)
		} else {
			buf = append(buf, "host"...)
			buf = appendPadded(buf, i, 8)
			buf = append(buf, " dom=host"...)
			buf = appendPadded(buf, i, 8)
			buf = append(buf, ".example.net"...)
		}
		// ~5% quoted values with spaces and # (the #14 stripComment path).
		if i%20 == 3 {
			buf = append(buf, ` note="`...)
			buf = append(buf, benchQuotedNote...)
			buf = append(buf, '"')
		}
		buf = append(buf, '\n')
		buf = append(buf, '\t')
		buf = append(buf, "owner="...)
		buf = append(buf, owner...)
		buf = append(buf, " loc=rack-"...)
		buf = strconv.AppendInt(buf, int64(i%64), 10)
		if i%10 == 4 {
			buf = append(buf, " dhcp"...)
		}
		for e := 0; e < extra; e++ {
			buf = append(buf, " extra="...)
			buf = strconv.AppendInt(buf, int64(e), 10)
		}
		if i%3 == 0 {
			buf = append(buf, " # trailing comment"...)
		}
		buf = append(buf, '\n')

		n, werr := w.Write(buf)
		if werr != nil {
			return st, werr
		}
		written += n
		st.records++
		st.sysHits++
	}
	if err := w.Flush(); err != nil {
		return st, err
	}
	fi, err := f.Stat()
	if err != nil {
		return st, err
	}
	st.size = fi.Size()
	return st, nil
}

func appendPadded(buf []byte, n, width int) []byte {
	var tmp [32]byte
	digits := strconv.AppendInt(tmp[:0], int64(n), 10)
	for i := len(digits); i < width; i++ {
		buf = append(buf, '0')
	}
	return append(buf, digits...)
}

// peakMem is sampled outside the timed loop so ReadMemStats (a stop-the-world
// call) does not inflate ns/op. Values are peak-minus-baseline after a GC
// taken immediately before the sampled pass, so the fixture already held in
// Ndb.data is not counted as scan growth.
//
//   - HeapInuse: live heap spans above that post-GC baseline. This is the
//     Go-level signal that Search/All started retaining every record.
//   - RSS: Linux resident set from /proc/self/statm, also as a delta. Zero
//     (and unreported) on non-Linux. Includes non-Go mappings.
type peakMem struct {
	HeapInuse uint64
	RSS       uint64
}

func updateMax(dst *atomic.Uint64, v uint64) {
	for {
		cur := dst.Load()
		if v <= cur || dst.CompareAndSwap(cur, v) {
			return
		}
	}
}

func readRSS() uint64 {
	data, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0
	}
	pages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0
	}
	return pages * uint64(os.Getpagesize())
}

func delta(peak, base uint64) uint64 {
	if peak < base {
		return 0
	}
	return peak - base
}

func measurePeak(fn func()) peakMem {
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	baseRSS := readRSS()

	var maxHeap, maxRSS atomic.Uint64
	maxHeap.Store(base.HeapInuse)
	if baseRSS > 0 {
		maxRSS.Store(baseRSS)
	}

	sample := func() {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		updateMax(&maxHeap, ms.HeapInuse)
		if rss := readRSS(); rss > 0 {
			updateMax(&maxRSS, rss)
		}
	}

	done := make(chan struct{})
	go func() {
		t := time.NewTicker(5 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				sample()
			}
		}
	}()

	fn()
	close(done)
	sample()

	out := peakMem{HeapInuse: delta(maxHeap.Load(), base.HeapInuse)}
	if baseRSS > 0 {
		out.RSS = delta(maxRSS.Load(), baseRSS)
	}
	return out
}

func reportPeak(b *testing.B, scan func()) {
	b.Helper()
	peak := measurePeak(scan)
	b.ReportMetric(float64(peak.HeapInuse), "peak-heap-B")
	if peak.RSS > 0 || readRSS() > 0 {
		// Report 0-delta RSS on Linux so the column stays present.
		b.ReportMetric(float64(peak.RSS), "peak-rss-B")
	}
}

func runSized(b *testing.B, scan func(*Ndb) int, want func(benchDB) int) {
	for _, sz := range allBenchSizes() {
		b.Run(sz.name, func(b *testing.B) {
			env := cachedBenchDB(b, sz)
			op := func() int { return scan(env.db) }
			b.SetBytes(env.size)
			b.ReportAllocs()
			b.ResetTimer()
			var last int
			for i := 0; i < b.N; i++ {
				last = op()
			}
			b.StopTimer()
			if got, exp := last, want(env); got != exp {
				b.Fatalf("result count=%d want %d", got, exp)
			}
			// Skip the N=1 calibration probe. A large scan's final N may
			// still be 1 when one pass already exceeds -benchtime.
			if sz.reportPeak && (b.N > 1 || sz.large) {
				reportPeak(b, func() { op() })
			}
		})
	}
}

// BenchmarkSearchHasAttr scans every record for a key present on all of them
// (Search + HasAttr) and reads one value with Get. This is the documented
// "search for a key" path.
func BenchmarkSearchHasAttr(b *testing.B) {
	pred := HasAttr("sys")
	runSized(b, func(db *Ndb) int {
		n := 0
		bytes := 0
		for rec := range db.Search(pred) {
			n++
			bytes += len(rec.Get("ip"))
		}
		benchSink = bytes
		return n
	}, func(env benchDB) int { return env.sysHits })
}

// BenchmarkSearchHasAttrMiss walks the file looking for a key that never
// appears. Matching fails before parseRecord, so this is raw key-scan cost.
func BenchmarkSearchHasAttrMiss(b *testing.B) {
	pred := HasAttr("no-such-attr")
	runSized(b, func(db *Ndb) int {
		n := 0
		for range db.Search(pred) {
			n++
		}
		benchSink = n
		return n
	}, func(benchDB) int { return 0 })
}

// BenchmarkSearchHasAttrValue finds one known sys= value with HasAttrValue
// (still a full file scan; Search does not stop after the first hit unless
// the caller breaks).
func BenchmarkSearchHasAttrValue(b *testing.B) {
	pred := HasAttrValue("sys", benchTargetSys)
	runSized(b, func(db *Ndb) int {
		n := 0
		bytes := 0
		for rec := range db.Search(pred) {
			n++
			bytes += len(rec.Get("sys"))
		}
		benchSink = bytes
		return n
	}, func(benchDB) int { return 1 })
}

// BenchmarkIterateAllRecords walks every record with All, the public
// record-iteration API.
func BenchmarkIterateAllRecords(b *testing.B) {
	runSized(b, func(db *Ndb) int {
		n := 0
		tuples := 0
		for rec := range db.All() {
			n++
			tuples += rec.Len()
		}
		benchSink = tuples
		return n
	}, func(env benchDB) int { return env.recs })
}

// BenchmarkIterateRecordsAndKeys walks every record with All and then every
// tuple with Len/KeyAt/ValueAt (the documented key-iteration API).
func BenchmarkIterateRecordsAndKeys(b *testing.B) {
	runSized(b, func(db *Ndb) int {
		n := 0
		bytes := 0
		for rec := range db.All() {
			n++
			for i := 0; i < rec.Len(); i++ {
				bytes += len(rec.KeyAt(i)) + len(rec.ValueAt(i))
			}
		}
		benchSink = bytes
		return n
	}, func(env benchDB) int { return env.recs })
}

func TestBenchFixturePublicAPI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bench.ndb")
	st, err := writeBenchFixture(path, 4<<10)
	if err != nil {
		t.Fatal(err)
	}
	if st.records < 2 {
		t.Fatalf("need at least 2 records, got %d", st.records)
	}
	if st.size <= 0 {
		t.Fatalf("empty fixture")
	}
	if st.sysHits != st.records {
		t.Fatalf("sysHits=%d records=%d", st.sysHits, st.records)
	}

	db, err := OpenOne(&LocalFileSystem{}, path)
	if err != nil {
		t.Fatal(err)
	}

	hits := db.SearchSlice(HasAttrValue("sys", benchTargetSys))
	if len(hits) != 1 {
		t.Fatalf("expected 1 %s record, got %d", benchTargetSys, len(hits))
	}
	if !hits[0].HasKey("ip") {
		t.Fatal("target record missing ip")
	}
	if hits[0].Get("sys") != benchTargetSys {
		t.Fatalf("target sys=%q", hits[0].Get("sys"))
	}

	quoted := db.SearchSlice(HasAttrValue("note", benchQuotedNote))
	if len(quoted) == 0 {
		t.Fatal("expected quoted note containing spaces and #")
	}
	if !strings.Contains(quoted[0].String(), `note="a b # c"`) {
		t.Fatalf("Record.String missing quoted hash: %q", quoted[0].String())
	}

	if len(db.SearchSlice(HasAttr("dhcp"))) == 0 {
		t.Fatal("expected bare dhcp attributes")
	}

	lens := map[int]int{}
	n := 0
	for rec := range db.All() {
		n++
		if rec.Len() == 0 {
			t.Fatal("empty record")
		}
		lens[rec.Len()]++
		_ = rec.KeyAt(0)
		_ = rec.ValueAt(0)
	}
	if n != st.records {
		t.Fatalf("All()=%d want %d", n, st.records)
	}
	if len(lens) < 2 {
		t.Fatalf("expected varied attribute counts, got lens %v", lens)
	}
	if got := len(db.SearchSlice(HasAttr("sys"))); got != st.sysHits {
		t.Fatalf("HasAttr(sys)=%d want %d", got, st.sysHits)
	}
	if got := len(db.SearchSlice(HasAttr("no-such-attr"))); got != 0 {
		t.Fatalf("miss search returned %d", got)
	}
}
