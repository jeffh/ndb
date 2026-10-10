package ndb

import (
	"bufio"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Deterministic fixture seed. Benches generate files at run time into
// b.TempDir(); nothing under testdata/ is committed.
const benchSeed int64 = 0x6e6462 // "ndb"

const (
	benchSizeSmall  = 64 << 10  // 64 KiB
	benchSizeMedium = 8 << 20   // 8 MiB
	benchSizeLarge  = 128 << 20 // 128 MiB; gated
	benchTargetSys  = "bench-target"
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
	db     *Ndb
	size   int64
	recs   int
	target string
}

func openBenchDB(b *testing.B, sz benchSize) benchDB {
	b.Helper()
	skipGatedSize(b, sz)

	path := filepath.Join(b.TempDir(), "bench.ndb")
	recs, size, err := writeBenchFixture(path, sz.target)
	if err != nil {
		b.Fatal(err)
	}
	db, err := OpenOne(&LocalFileSystem{}, path)
	if err != nil {
		b.Fatal(err)
	}
	return benchDB{db: db, size: size, recs: recs, target: benchTargetSys}
}

func writeBenchFixture(path string, targetBytes int) (records int, size int64, err error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, 0, err
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
			buf = append(buf, " dom=target.example.net\n"...)
		} else {
			buf = append(buf, "host"...)
			buf = appendPadded(buf, i, 8)
			buf = append(buf, " dom=host"...)
			buf = appendPadded(buf, i, 8)
			buf = append(buf, ".example.net\n"...)
		}
		buf = append(buf, '\t')
		buf = append(buf, "owner="...)
		buf = append(buf, owner...)
		buf = append(buf, " loc=rack-"...)
		buf = strconv.AppendInt(buf, int64(i%64), 10)
		buf = append(buf, '\n')

		n, werr := w.Write(buf)
		if werr != nil {
			return records, 0, werr
		}
		written += n
		records++
	}
	if err := w.Flush(); err != nil {
		return records, 0, err
	}
	st, err := f.Stat()
	if err != nil {
		return records, 0, err
	}
	return records, st.Size(), nil
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
// call) does not inflate ns/op.
//
//   - HeapInuse: bytes in in-use heap spans. This is the best Go-level signal
//     that Search/All started retaining every record instead of reusing one.
//   - Sys: bytes the runtime has obtained from the OS (reserved, not all RSS).
//   - RSS: Linux resident set from /proc/self/statm (resident pages × page size).
//     Zero on non-Linux. RSS includes non-Go mappings; treat it as an upper bound
//     on process footprint, not as a per-op allocation count.
type peakMem struct {
	HeapInuse uint64
	Sys       uint64
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

func measurePeak(fn func()) peakMem {
	var maxHeap, maxSys, maxRSS atomic.Uint64
	sample := func() {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		updateMax(&maxHeap, ms.HeapInuse)
		updateMax(&maxSys, ms.Sys)
		if rss := readRSS(); rss > 0 {
			updateMax(&maxRSS, rss)
		}
	}

	runtime.GC()
	sample()

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
	return peakMem{HeapInuse: maxHeap.Load(), Sys: maxSys.Load(), RSS: maxRSS.Load()}
}

func reportPeak(b *testing.B, scan func()) {
	b.Helper()
	peak := measurePeak(scan)
	b.ReportMetric(float64(peak.HeapInuse), "peak-heap-B")
	b.ReportMetric(float64(peak.Sys), "peak-sys-B")
	if peak.RSS > 0 {
		b.ReportMetric(float64(peak.RSS), "peak-rss-B")
	}
}

func runSized(b *testing.B, scan func(*Ndb)) {
	for _, sz := range allBenchSizes() {
		b.Run(sz.name, func(b *testing.B) {
			env := openBenchDB(b, sz)
			op := func() { scan(env.db) }
			b.SetBytes(env.size)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				op()
			}
			b.StopTimer()
			if sz.reportPeak {
				reportPeak(b, op)
			}
		})
	}
}

// BenchmarkSearchHasAttr scans every record for a key present on all of them
// (Search + HasAttr) and reads one value with Get. This is the documented
// "search for a key" path.
func BenchmarkSearchHasAttr(b *testing.B) {
	pred := HasAttr("sys")
	runSized(b, func(db *Ndb) {
		n := 0
		for rec := range db.Search(pred) {
			n += len(rec.Get("ip"))
		}
		benchSink = n
	})
}

// BenchmarkSearchHasAttrMiss walks the file looking for a key that never
// appears. Matching fails before parseRecord, so this is raw key-scan cost.
func BenchmarkSearchHasAttrMiss(b *testing.B) {
	pred := HasAttr("no-such-attr")
	runSized(b, func(db *Ndb) {
		n := 0
		for range db.Search(pred) {
			n++
		}
		benchSink = n
	})
}

// BenchmarkSearchHasAttrValue finds one known sys= value with HasAttrValue
// (still a full file scan; Search does not stop after the first hit unless
// the caller breaks).
func BenchmarkSearchHasAttrValue(b *testing.B) {
	pred := HasAttrValue("sys", benchTargetSys)
	runSized(b, func(db *Ndb) {
		n := 0
		for rec := range db.Search(pred) {
			n += len(rec.Get("sys"))
		}
		benchSink = n
	})
}

// BenchmarkIterateAllRecords walks every record with All, the public
// record-iteration API.
func BenchmarkIterateAllRecords(b *testing.B) {
	runSized(b, func(db *Ndb) {
		n := 0
		for rec := range db.All() {
			n += rec.Len()
		}
		benchSink = n
	})
}

// BenchmarkIterateRecordsAndKeys walks every record with All and then every
// tuple with Len/KeyAt/ValueAt (the documented key-iteration API).
func BenchmarkIterateRecordsAndKeys(b *testing.B) {
	runSized(b, func(db *Ndb) {
		n := 0
		for rec := range db.All() {
			for i := 0; i < rec.Len(); i++ {
				n += len(rec.KeyAt(i)) + len(rec.ValueAt(i))
			}
		}
		benchSink = n
	})
}

func TestBenchFixturePublicAPI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bench.ndb")
	recs, size, err := writeBenchFixture(path, 4<<10)
	if err != nil {
		t.Fatal(err)
	}
	if recs < 2 {
		t.Fatalf("need at least 2 records, got %d", recs)
	}
	if size <= 0 {
		t.Fatalf("empty fixture")
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

	n := 0
	for rec := range db.All() {
		n++
		if rec.Len() == 0 {
			t.Fatal("empty record")
		}
		_ = rec.KeyAt(0)
		_ = rec.ValueAt(0)
	}
	if n != recs {
		t.Fatalf("All()=%d want %d", n, recs)
	}
	if got := len(db.SearchSlice(HasAttr("sys"))); got != recs {
		t.Fatalf("HasAttr(sys)=%d want %d", got, recs)
	}
	if got := len(db.SearchSlice(HasAttr("no-such-attr"))); got != 0 {
		t.Fatalf("miss search returned %d", got)
	}
}
