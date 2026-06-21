package kube

import (
	"context"
	"reflect"
	"testing"
)

type listGetter struct{ out string }

func (g listGetter) Get(context.Context, ...string) ([]byte, error) { return []byte(g.out), nil }

func TestListBattleGroupNamespaces(t *testing.T) {
	g := listGetter{out: "funcom-operators\nfuncom-seabass-abc\nkube-system\nfuncom-seabass-xyz\n"}
	got, err := ListBattleGroupNamespaces(context.Background(), g)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []string{"funcom-seabass-abc", "funcom-seabass-xyz"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestListBattleGroupNamespaces_Empty(t *testing.T) {
	got, err := ListBattleGroupNamespaces(context.Background(), listGetter{out: "kube-system\n"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want none, got %v", got)
	}
}
