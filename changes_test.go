package main

import (
	"fmt"
	"testing"
)

func TestNewDiff(t *testing.T) {
	diff, err := NewDiff("testdata/openrpc_old.json", "testdata/openrpc_new.json", Options{})

	if err != nil {
		t.Fatalf("new diff error: %s", err)
	}

	fmt.Println(diff.String())
}
