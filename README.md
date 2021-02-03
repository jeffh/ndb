ndb
======

Implements the [Plan9 Network Database](https://9fans.github.io/plan9port/man/man7/ndb.html).

Usage:

```go
db, err := ndb.Read(ctx, "file.db")
if err != nil { ... }

db.M.Lock()
for _, rec := range db.Records {
    for i, size := 0, rec.Len(); i < size; i++ {
        key := rec.KeyAt(i)
        value := rec.ValueAt(i)
        fmt.Printf("%s=%s ", key, value)
    }
    fmt.Printf("\n")
}
db.M.Unlock()

db.Save(ctx, nil)
```
