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

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	ctx := context.Background()
	filename := flag.Arg(0)
	if flag.NArg() >= 2 {
		key := flag.Arg(1)
		var value string
		if flag.NArg() > 2 {
			value = flag.Arg(2)
		}
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
	} else {
		db, err := ndb.Open(ctx, filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open: %s", err)
			os.Exit(2)
		}

		it := db.SearchPredicate(func(r *ndb.Record) bool { return true })
		defer it.Close()
		for it.Next() {
			r := it.Record()
			fmt.Printf(" - %s\n", r.String())
		}
	}
}
