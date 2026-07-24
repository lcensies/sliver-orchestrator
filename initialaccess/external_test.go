package initialaccess

import (
	"context"
	"testing"
)

func TestExternalModule_JSONResult(t *testing.T) {
	m := &ExternalModule{}
	// A shell that echoes a JSON Result on stdout.
	req := Request{
		Target: Target{Name: "web1", Host: "10.0.0.1"},
		Config: map[string]string{
			"shell": `echo '{"ok":true,"note":"pwned","hostname":"victim-web"}'`,
		},
	}
	res, err := m.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Ok || res.Note != "pwned" || res.Hostname != "victim-web" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestExternalModule_ExitCodeInference(t *testing.T) {
	m := &ExternalModule{}
	// Non-JSON stdout, exit 0 => Ok true.
	res, err := m.Run(context.Background(), Request{Config: map[string]string{"shell": "echo hello"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Ok || res.Note != "hello" {
		t.Fatalf("expected ok+hello, got %+v", res)
	}

	// Non-zero exit => Ok false.
	res, err = m.Run(context.Background(), Request{Config: map[string]string{"shell": "exit 3"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Ok {
		t.Fatalf("expected failure on exit 3, got %+v", res)
	}
}

func TestExternalModule_RunArgvJSON(t *testing.T) {
	m := &ExternalModule{}
	req := Request{Config: map[string]string{"run": `["sh","-c","echo {\"ok\":true}"]`}}
	res, err := m.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Ok {
		t.Fatalf("expected ok, got %+v", res)
	}
}

func TestExternalModule_MissingRun(t *testing.T) {
	m := &ExternalModule{}
	if _, err := m.Run(context.Background(), Request{Config: map[string]string{}}); err == nil {
		t.Fatal("expected error when neither run nor shell is set")
	}
}

func TestExternalModule_BinaryNotFound(t *testing.T) {
	m := &ExternalModule{}
	_, err := m.Run(context.Background(), Request{Config: map[string]string{"run": "/no/such/binary-xyz"}})
	if err == nil {
		t.Fatal("expected transport error when binary cannot start")
	}
}

func TestRegistry(t *testing.T) {
	r := DefaultRegistry()
	if _, err := r.Get("external"); err != nil {
		t.Fatalf("external module should be registered: %v", err)
	}
	if _, err := r.Get("nope"); err == nil {
		t.Fatal("expected error for unknown module")
	}
}
