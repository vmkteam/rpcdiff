package main

import (
	"testing"
)

func TestNewDiff(t *testing.T) {
	diff, err := NewDiff("testdata/openrpc_old.json", "testdata/openrpc_new.json", Options{})

	if err != nil {
		t.Fatalf("new diff error: %s", err)
	}

	changesMap := map[CriticalityLevel][]Change{
		Breaking:    {},
		Dangerous:   {},
		NonBreaking: {},
	}

	for _, change := range diff.Changes {
		changesMap[change.Criticality] = append(changesMap[change.Criticality], change)
	}

	if diff.Criticality != Breaking {
		t.Fatalf("diff.Criticality = %v, wanted %v", diff.Criticality, Breaking)
	}

	if len(diff.Changes) != 17 {
		t.Fatalf("len(diff.Changes) = %v, wanted %v", len(diff.Changes), 17)
	}

	if len(changesMap[Breaking]) != 7 {
		t.Fatalf("len %s changes = %v, wanted %v", Breaking, len(changesMap[Breaking]), 7)
	}

	if len(changesMap[Dangerous]) != 1 {
		t.Fatalf("len %s changes = %v, wanted %v", Dangerous, len(changesMap[Dangerous]), 7)
	}

	if len(changesMap[NonBreaking]) != 9 {
		t.Fatalf("len %s changes = %v, wanted %v", NonBreaking, len(changesMap[NonBreaking]), 7)
	}

	//fmt.Println(diff.String())
}
