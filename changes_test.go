package main

import (
	"fmt"
	"testing"

	"github.com/vmkteam/zenrpc/v2"
	"github.com/vmkteam/zenrpc/v2/testdata"
)

func TestNewDiff(t *testing.T) {
	rpc := zenrpc.NewServer(zenrpc.Options{})
	rpc.Register("arith", testdata.ArithService{})

	diff, err := NewDiff("testdata/openrpc_old.json", "testdata/openrpc_new.json")

	if err != nil {
		t.Fatalf("new diff error: %s", err)
	}

	fmt.Println(diff.String())
}
