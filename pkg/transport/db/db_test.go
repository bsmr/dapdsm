package db

import (
	"context"
	"reflect"
	"testing"
)

type recExecer struct {
	gotNS, gotPod string
	gotStdin      []byte
	gotCmd        []string
	out           string
}

func (r *recExecer) Run(_ context.Context, ns, pod string, stdin []byte, command ...string) (string, error) {
	r.gotNS, r.gotPod, r.gotStdin, r.gotCmd = ns, pod, stdin, command
	return r.out, nil
}

func TestRunBuildsPipedQueryArgv(t *testing.T) {
	rec := &recExecer{out: "42\n"}
	c := &Conn{
		Creds: Creds{Namespace: "ns", Pod: "p-sts-0", Port: 15432, SuperUser: "postgres", SuperPassword: "pw", Database: "dune"},
		Exec:  rec,
	}
	out, err := c.Run(context.Background(), Query{Password: true, SearchPath: true, Tuples: true, SQL: "SELECT 1;"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "42\n" {
		t.Fatalf("out = %q", out)
	}
	if string(rec.gotStdin) != "SELECT 1;" {
		t.Fatalf("stdin = %q", rec.gotStdin)
	}
	want := []string{
		"env", "PGPASSWORD=pw", "PGOPTIONS=-c search_path=dune,public",
		"psql", "-h", "127.0.0.1", "-p", "15432", "-U", "postgres", "-d", "dune",
		"-tA", "-F", "|", "-f", "-",
	}
	if !reflect.DeepEqual(rec.gotCmd, want) {
		t.Fatalf("cmd =\n %v\nwant\n %v", rec.gotCmd, want)
	}
}

func TestRunInlineAndVarsAndOverrides(t *testing.T) {
	rec := &recExecer{}
	c := &Conn{Creds: Creds{Pod: "p-sts-0", Port: 15432, SuperUser: "postgres", Database: "dune"}, Exec: rec}
	_, err := c.Run(context.Background(), Query{
		Database: "postgres", User: "super", Tuples: true,
		Vars:   []Var{{"a", "1"}, {"b", "2"}},
		Inline: "SELECT now();",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.gotStdin != nil {
		t.Fatalf("inline must not send stdin, got %q", rec.gotStdin)
	}
	want := []string{
		"psql", "-h", "127.0.0.1", "-p", "15432", "-U", "super", "-d", "postgres",
		"-tA", "-F", "|", "-v", "a=1", "-v", "b=2", "-c", "SELECT now();",
	}
	if !reflect.DeepEqual(rec.gotCmd, want) {
		t.Fatalf("cmd =\n %v\nwant\n %v", rec.gotCmd, want)
	}
}
