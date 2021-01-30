package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/jeffh/ndb"
)

func main() {
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Usage: %s DB KEY VALUE\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(1)
	}

	filename := flag.Arg(0)
	key := flag.Arg(1)
	var value string
	if flag.NArg() > 2 {
		value = flag.Arg(2)
	}
	ctx := context.Background()
	db, err := ndb.Open(ctx, filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open: %s", err)
		os.Exit(2)
	}

	it := db.Search(key, value)
	defer it.Close()
	for it.Next() {
		r := it.Record()
		fmt.Printf(" - %s\n", r.String())
	}
}
