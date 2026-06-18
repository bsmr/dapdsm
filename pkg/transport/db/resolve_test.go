package db

import (
	"context"
	"testing"
)

type fakeGetter struct{ json string }

func (f fakeGetter) Get(_ context.Context, _ ...string) ([]byte, error) { return []byte(f.json), nil }

func TestResolveDecodesCR(t *testing.T) {
	g := fakeGetter{json: `{"items":[{"metadata":{"name":"bgdb","namespace":"ns7"},"spec":{"port":15432,"superUser":"postgres","superPassword":"sp","user":"game","password":"gp","gameDatabaseName":"dune"}}]}`}
	c, err := Resolve(context.Background(), g, "ns7")
	if err != nil {
		t.Fatal(err)
	}
	if c.Pod != "bgdb-sts-0" || c.Namespace != "ns7" || c.Port != 15432 ||
		c.SuperUser != "postgres" || c.SuperPassword != "sp" || c.GameUser != "game" ||
		c.GamePassword != "gp" || c.Database != "dune" {
		t.Fatalf("creds = %+v", c)
	}
}

func TestResolveEmptyIsError(t *testing.T) {
	if _, err := Resolve(context.Background(), fakeGetter{json: `{"items":[]}`}, "ns"); err == nil {
		t.Fatal("want error on no items")
	}
}
