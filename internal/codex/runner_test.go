package codex

import (
	"testing"
	"time"
)

func TestBuildArgs_Basic(t *testing.T) {
	req := ExecRequest{
		Model:      "gpt-5.3-Codex",
		Prompt:     "Review this code",
		SchemaPath: "/tmp/schema.json",
	}

	args := BuildArgs(req)

	assertArgsContain(t, args, "exec")
	assertArgsContain(t, args, "-m")
	assertArgsContain(t, args, "gpt-5.3-Codex")
	assertArgsContain(t, args, "--output-schema")
	assertArgsContain(t, args, "/tmp/schema.json")
	assertArgsContain(t, args, "--skip-git-repo-check")
	assertArgsContain(t, args, "skills.allow_implicit_invocation=false")
	assertArgsContain(t, args, "Review this code")
}

func TestBuildArgs_WithResume(t *testing.T) {
	req := ExecRequest{
		Model:           "gpt-5.3-Codex",
		Prompt:          "Verify fixes",
		SchemaPath:      "/tmp/schema.json",
		ResumeSessionID: "abc-123-def",
	}

	args := BuildArgs(req)

	assertArgsContain(t, args, "-r")
	assertArgsContain(t, args, "abc-123-def")
}

func TestBuildArgs_NoResume(t *testing.T) {
	req := ExecRequest{
		Model:  "gpt-5.3-Codex",
		Prompt: "Review",
	}

	args := BuildArgs(req)

	for _, arg := range args {
		if arg == "-r" {
			t.Error("should not contain -r when ResumeSessionID is empty")
		}
	}
}

func TestBuildArgs_NoModel(t *testing.T) {
	req := ExecRequest{
		Prompt: "Review",
	}

	args := BuildArgs(req)

	for _, arg := range args {
		if arg == "-m" {
			t.Error("should not contain -m when Model is empty")
		}
	}
}

func TestBuildArgs_NoSchema(t *testing.T) {
	req := ExecRequest{
		Prompt: "Review",
	}

	args := BuildArgs(req)

	for _, arg := range args {
		if arg == "--output-schema" {
			t.Error("should not contain --output-schema when SchemaPath is empty")
		}
	}
}

func TestBuildArgs_PromptIsLastArg(t *testing.T) {
	req := ExecRequest{
		Model:  "gpt-5.3-Codex",
		Prompt: "Review this code",
	}

	args := BuildArgs(req)

	last := args[len(args)-1]
	if last != "Review this code" {
		t.Errorf("expected prompt as last arg, got %q", last)
	}
}

func TestExecRequest_TimeoutField(t *testing.T) {
	req := ExecRequest{
		Timeout: 180 * time.Second,
	}
	if req.Timeout != 180*time.Second {
		t.Errorf("expected 180s, got %s", req.Timeout)
	}
}

func assertArgsContain(t *testing.T, args []string, want string) {
	t.Helper()
	for _, a := range args {
		if a == want {
			return
		}
	}
	t.Errorf("args %v does not contain %q", args, want)
}
