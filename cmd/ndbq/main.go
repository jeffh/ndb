package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jeffh/ndb"
)

func main() {
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Usage: %s DB [KEY [VALUE]]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	filename := flag.Arg(0)
	fs := &ndb.LocalFileSystem{}
	db, err := ndb.Open(fs, filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open: %s\n", err)
		os.Exit(2)
	}

	if flag.NArg() >= 2 {
		key := flag.Arg(1)
		var value string
		if flag.NArg() > 2 {
			value = flag.Arg(2)
		}

		for r := range db.Search(ndb.HasAttrValue(key, value)) {
			fmt.Printf(" - %s\n", r.String())
		}
	} else {
		for r := range db.All() {
			fmt.Printf(" - %s\n", r.String())
		}
	}
}
