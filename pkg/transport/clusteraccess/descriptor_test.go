package clusteraccess

import "testing"

const sampleInventory = `
all:
  children:
    controlplane:
      hosts:
        c-0: {ansible_host: 10.0.0.5}
        c-1: {ansible_host: 10.0.0.6}
    worker:
      hosts:
        w-0: {ansible_host: 10.0.0.8}
    lb:
      hosts:
        lb-0: {ansible_host: 10.0.0.2}
  vars:
    ansible_user: installer
    ansible_ssh_private_key_file: /home/installer/.keys/id_ed25519
`

func TestParseInventory(t *testing.T) {
	nodes, user, key, err := parseInventory([]byte(sampleInventory))
	if err != nil {
		t.Fatalf("parseInventory: %v", err)
	}
	if user != "installer" {
		t.Errorf("user = %q, want installer", user)
	}
	if key != "/home/installer/.keys/id_ed25519" {
		t.Errorf("key = %q", key)
	}
	if len(nodes) != 4 {
		t.Fatalf("len(nodes) = %d, want 4", len(nodes))
	}
	// sorted by name: c-0, c-1, lb-0, w-0
	type want struct {
		name    string
		role    Role
		address string
	}
	expected := []want{
		{"c-0", RoleControlPlane, "10.0.0.5"},
		{"c-1", RoleControlPlane, "10.0.0.6"},
		{"lb-0", RoleLB, "10.0.0.2"},
		{"w-0", RoleWorker, "10.0.0.8"},
	}
	for i, e := range expected {
		if nodes[i].Name != e.name || nodes[i].Role != e.role || nodes[i].Address != e.address {
			t.Errorf("nodes[%d] = %+v, want {Name:%s Role:%v Address:%s}", i, nodes[i], e.name, e.role, e.address)
		}
	}
}

func TestParseInventory_Empty(t *testing.T) {
	// "all: {}" is non-empty YAML that yields zero nodes; the guard rejects it.
	if _, _, _, err := parseInventory([]byte("all: {}")); err == nil {
		t.Fatal("want error for non-empty nodeless inventory, got nil")
	}
}

func TestParseInventory_BadYAML(t *testing.T) {
	if _, _, _, err := parseInventory([]byte("\t not: : yaml")); err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}
