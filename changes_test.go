package main

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/vmkteam/zenrpc/v2"
	"github.com/vmkteam/zenrpc/v2/testdata"
)

func TestNewDiff(t *testing.T) {
	rpc := zenrpc.NewServer(zenrpc.Options{})
	rpc.Register("arith", testdata.ArithService{})

	oldJSON, err := ioutil.ReadFile("testdata/openrpc_old.json")
	if err != nil {
		t.Fatalf("read old data error: %s", err)
	}

	newJSON, err := ioutil.ReadFile("testdata/openrpc_new.json")
	if err != nil {
		t.Fatalf("read new data error: %s", err)
	}

	diff, err := NewDiff(oldJSON, newJSON)

	if err != nil {
		t.Fatalf("new diff error: %s", err)
	}

	fmt.Println(diff.String())
}
