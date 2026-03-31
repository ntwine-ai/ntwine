package harness

import (
	"strings"
	"testing"
)

var sampleAgent = AgentRole{
	ID:          "alice-id",
	Name:        "Alice",
	ModelID:     "gpt-4",
	Role:        "backend developer",
	Description: "specializes in Go and databases",
}

var sampleTeammate = AgentRole{
	ID:   "bob-id",
	Name: "Bob",
	Role: "frontend developer",
}

// BuildSystemPrompt

func TestBuildSystemPrompt_ContainsAgentName(t *testing.T) {
	prompt := BuildSystemPrompt(sampleAgent, nil, "/project", "")
	if !strings.Contains(prompt, "Alice") {
		t.Errorf("expected agent name in prompt, got: %q", prompt[:100])
	}
}

func TestBuildSystemPrompt_ContainsRole(t *testing.T) {
	prompt := BuildSystemPrompt(sampleAgent, nil, "/project", "")
	if !strings.Contains(prompt, "backend developer") {
		t.Errorf("expected role in prompt, got: %q", prompt[:200])
	}
}

func TestBuildSystemPrompt_ContainsDescription(t *testing.T) {
	prompt := BuildSystemPrompt(sampleAgent, nil, "/project", "")
	if !strings.Contains(prompt, "specializes in Go and databases") {
		t.Errorf("expected description in prompt")
	}
}

func TestBuildSystemPrompt_ListsTeammates(t *testing.T) {
	prompt := BuildSystemPrompt(sampleAgent, []AgentRole{sampleAgent, sampleTeammate}, "/project", "")
	if !strings.Contains(prompt, "Bob") {
		t.Errorf("expected teammate Bob in prompt")
	}
}

func TestBuildSystemPrompt_ExcludesSelf(t *testing.T) {
	prompt := BuildSystemPrompt(sampleAgent, []AgentRole{sampleAgent, sampleTeammate}, "/project", "")
	// Alice should appear once as "you are Alice" but NOT in the team list
	teamSection := strings.Index(prompt, "your team:")
	if teamSection == -1 {
		t.Skip("no team section found")
	}
	teamPart := prompt[teamSection:]
	// "Alice" should not appear as a teammate entry "- Alice"
	if strings.Contains(teamPart, "- Alice") {
		t.Error("agent should not list themselves as a teammate")
	}
}

func TestBuildSystemPrompt_NoTeammates_NoTeamSection(t *testing.T) {
	prompt := BuildSystemPrompt(sampleAgent, nil, "/project", "")
	if strings.Contains(prompt, "your team:") {
		t.Error("should not have team section with no teammates")
	}
}

func TestBuildSystemPrompt_ContainsCodebasePath(t *testing.T) {
	prompt := BuildSystemPrompt(sampleAgent, nil, "/my/project/path", "")
	if !strings.Contains(prompt, "/my/project/path") {
		t.Errorf("expected codebase path in prompt")
	}
}

func TestBuildSystemPrompt_EmptyCodebasePath_NoProjectLine(t *testing.T) {
	prompt := BuildSystemPrompt(sampleAgent, nil, "", "")
	if strings.Contains(prompt, "project: \n") {
		t.Error("empty codebase path should not produce empty project line")
	}
}

func TestBuildSystemPrompt_WithSpec(t *testing.T) {
	spec := "## Goals\nBuild a REST API"
	prompt := BuildSystemPrompt(sampleAgent, nil, "/proj", spec)
	if !strings.Contains(prompt, "Build a REST API") {
		t.Errorf("expected spec content in prompt")
	}
	if !strings.Contains(prompt, "current spec:") {
		t.Errorf("expected 'current spec:' header in prompt")
	}
}

func TestBuildSystemPrompt_NoSpec_NoSpecSection(t *testing.T) {
	prompt := BuildSystemPrompt(sampleAgent, nil, "/proj", "")
	if strings.Contains(prompt, "current spec:") {
		t.Error("should not have spec section when spec is empty")
	}
}

func TestBuildSystemPrompt_ContainsRules(t *testing.T) {
	prompt := BuildSystemPrompt(sampleAgent, nil, "", "")
	// should have tool usage rules
	if !strings.Contains(prompt, "read files before editing") {
		t.Error("expected tool usage rules in prompt")
	}
}

func TestBuildSystemPrompt_TeammateDefaultRole(t *testing.T) {
	noRole := AgentRole{ID: "c-id", Name: "Charlie"}
	prompt := BuildSystemPrompt(sampleAgent, []AgentRole{sampleAgent, noRole}, "", "")
	if !strings.Contains(prompt, "developer") {
		t.Errorf("expected default 'developer' role for teammate without role")
	}
}

// BuildPlanningPrompt

func TestBuildPlanningPrompt_ContainsAgentName(t *testing.T) {
	prompt := BuildPlanningPrompt(sampleAgent, nil, "build a cache", "/proj")
	if !strings.Contains(prompt, "Alice") {
		t.Errorf("expected agent name in planning prompt")
	}
}

func TestBuildPlanningPrompt_ContainsTask(t *testing.T) {
	prompt := BuildPlanningPrompt(sampleAgent, nil, "implement caching layer", "/proj")
	if !strings.Contains(prompt, "implement caching layer") {
		t.Errorf("expected task in planning prompt")
	}
}

func TestBuildPlanningPrompt_ContainsCodebasePath(t *testing.T) {
	prompt := BuildPlanningPrompt(sampleAgent, nil, "task", "/the/project")
	if !strings.Contains(prompt, "/the/project") {
		t.Errorf("expected codebase path in planning prompt")
	}
}

func TestBuildPlanningPrompt_ListsTeammates(t *testing.T) {
	prompt := BuildPlanningPrompt(sampleAgent, []AgentRole{sampleAgent, sampleTeammate}, "task", "")
	if !strings.Contains(prompt, "Bob") {
		t.Errorf("expected Bob in planning prompt team section")
	}
}

func TestBuildPlanningPrompt_ContainsPlanningRules(t *testing.T) {
	prompt := BuildPlanningPrompt(sampleAgent, nil, "task", "")
	if !strings.Contains(prompt, "planning rules:") {
		t.Error("expected planning rules in prompt")
	}
}

func TestBuildPlanningPrompt_EmptyCodebasePath(t *testing.T) {
	// should not panic or produce malformed output
	prompt := BuildPlanningPrompt(sampleAgent, nil, "task", "")
	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
}

func TestBuildPlanningPrompt_TeammateDefaultRole(t *testing.T) {
	noRole := AgentRole{ID: "z-id", Name: "Zara"}
	prompt := BuildPlanningPrompt(sampleAgent, []AgentRole{sampleAgent, noRole}, "task", "")
	if !strings.Contains(prompt, "developer") {
		t.Errorf("expected default 'developer' role for teammate without role, got: %q", prompt[strings.Index(prompt, "team:"):])
	}
}

// BuildReviewPrompt

func TestBuildReviewPrompt_ContainsAgentName(t *testing.T) {
	prompt := BuildReviewPrompt(sampleAgent, nil, "/proj")
	if !strings.Contains(prompt, "Alice") {
		t.Errorf("expected agent name in review prompt")
	}
}

func TestBuildReviewPrompt_ContainsCodebasePath(t *testing.T) {
	prompt := BuildReviewPrompt(sampleAgent, nil, "/the/code")
	if !strings.Contains(prompt, "/the/code") {
		t.Errorf("expected codebase path in review prompt")
	}
}

func TestBuildReviewPrompt_ContainsReviewRules(t *testing.T) {
	prompt := BuildReviewPrompt(sampleAgent, nil, "")
	if !strings.Contains(prompt, "review rules:") {
		t.Error("expected review rules in prompt")
	}
}

func TestBuildReviewPrompt_EmptyPath(t *testing.T) {
	prompt := BuildReviewPrompt(sampleAgent, nil, "")
	if prompt == "" {
		t.Error("expected non-empty review prompt")
	}
}

// BuildCriticPrompt

func TestBuildCriticPrompt_ContainsAgentName(t *testing.T) {
	prompt := BuildCriticPrompt(sampleAgent, "/proj")
	if !strings.Contains(prompt, "Alice") {
		t.Errorf("expected agent name in critic prompt")
	}
}

func TestBuildCriticPrompt_ContainsAdversarialNote(t *testing.T) {
	prompt := BuildCriticPrompt(sampleAgent, "/proj")
	if !strings.Contains(prompt, "adversarial") {
		t.Errorf("expected adversarial mention in critic prompt")
	}
}

func TestBuildCriticPrompt_ContainsCriticRules(t *testing.T) {
	prompt := BuildCriticPrompt(sampleAgent, "")
	if !strings.Contains(prompt, "critic rules:") {
		t.Error("expected critic rules in prompt")
	}
}

func TestBuildCriticPrompt_ContainsCodebasePath(t *testing.T) {
	prompt := BuildCriticPrompt(sampleAgent, "/my/codebase")
	if !strings.Contains(prompt, "/my/codebase") {
		t.Errorf("expected codebase path in critic prompt")
	}
}

func TestBuildCriticPrompt_EmptyPath(t *testing.T) {
	prompt := BuildCriticPrompt(sampleAgent, "")
	if prompt == "" {
		t.Error("expected non-empty critic prompt")
	}
	if strings.Contains(prompt, "project: \n") {
		t.Error("empty path should not produce empty project line")
	}
}
