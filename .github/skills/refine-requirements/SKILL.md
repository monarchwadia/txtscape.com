---
name: refine-requirements
description: "Refine a requirement into a complete spec. USE WHEN: user says 'refine', 'requirements', 'spec', 'requirement', 'flesh out', 'define', 'scope', 'design', 'user story', 'what should this look like'. Supports: /refine-requirements, /refine-requirements <topic>. Transforms a rough idea into business context, user stories, and UX design reasoning."
---

# /refine-requirements

Transform a rough requirement or feature idea into a structured, complete specification. Read [Project.md](../../../Project.md) for full project context.

## Step 1: Deep-read the journal

Before refining anything, thoroughly mine the txtscape journal for context. This is read-only — never write to the journal from this skill.

1. Call `search_pages` with the topic and related keywords (e.g. for "search": search, find, query, filter)
2. Call `list_pages` on every concern folder: `decisions/`, `architecture/`, `todos/`, `roadmap/`, `learnings/`, `users/`, `references/`
3. Read all relevant pages with `get_page` — err on the side of reading too much
4. Call `related_pages` on any page that seems central to the requirement
5. Use `snapshot` if the journal is small enough to load entirely

### What to extract

- **Decisions** that constrain the solution (from `decisions/`)
- **Architecture** that the feature must fit into (from `architecture/`)
- **User signals** that validate or shape the requirement (from `users/`)
- **Learnings** that inform design choices (from `learnings/`)
- **Existing plans** that this overlaps with or depends on (from `todos/`, `roadmap/`)
- **Reference material** relevant to the domain (from `references/`)

Surface any conflicts or dependencies found — the refined requirement must not contradict recorded decisions.

If txtscape has no relevant pages, note this and proceed.

## Step 2: Gather the raw input

Identify what the user has given you — it might be:

- A one-liner ("add search to the UI")
- A todo page in the journal
- A conversation about a feature
- A vague aspiration ("make it easier to find things")

If the input is too vague to reason about, ask clarifying questions using the ask-questions tool. Ask at most 3 questions, focused on:

1. **Who** is the user/persona affected?
2. **What problem** are they experiencing today?
3. **What does success look like** from their perspective?

## Step 3: Write the business requirements

Structure the **why** before the **what**. Output this section:

```
## Business Requirements

### Problem Statement
[What pain or gap exists today. Be specific — quote user signals, observed behavior, or known friction.]

### Why This Matters
[Strategic justification. How does solving this connect to the product's goals? Why now?]

### Success Criteria
[Observable outcomes that prove the requirement is met. Not implementation details — user-visible results.]
- [ ] [Criterion 1]
- [ ] [Criterion 2]
- [ ] [Criterion 3]
```

## Step 4: Define strategic goals and tactical context

Provide the context an LLM implementer would need to make good decisions:

```
## Strategic & Tactical Context

### Strategic Goals
[How this feature fits into the broader product direction. What doors does it open? What bets does it represent?]

### Constraints
[Technical, timeline, or design constraints that bound the solution space.]
- [Constraint 1]
- [Constraint 2]

### Dependencies
[What must exist before this can be built? What does this unlock for later work?]

### Non-Goals
[What this requirement explicitly does NOT cover. Prevent scope creep by naming it.]
```

## Step 5: Write user stories

Capture the product-level behavior from the user's perspective:

```
## User Stories

### Primary Story
As a [persona], I want to [action] so that [outcome].

### Acceptance Criteria
Given [context], when [action], then [expected result].
Given [context], when [action], then [expected result].

### Secondary Stories
[Additional user stories for edge cases, alternate flows, or related personas.]
```

Ground stories in real usage patterns. Quote or cite specific txtscape `users/` pages and `learnings/` entries found in Step 1. If none exist, note that the stories are based on inference rather than observed signals.

## Step 6: Write the high-level design story

Describe the technical approach at the architecture level — NOT implementation details:

```
## High-Level Design

### Approach
[How the feature fits into the existing architecture. Which layers it touches, what the data flow looks like, where the boundaries are.]

### Key Design Decisions
[Decisions that shape the implementation. For each, state the decision and the rationale.]
- **[Decision]**: [Rationale]

### Risks & Open Questions
- [Risk or question that needs answers before or during implementation]
```

Reference specific `architecture/` and `decisions/` pages found in Step 1 by name. State how each referenced decision or architectural constraint shapes the design.

## Step 7: Reason about visual design and UX

Think through what the feature should look like and feel like from the user's perspective:

```
## Visual & UX Design

### User Flow
[Step-by-step walkthrough of the user's journey. What do they see first? What do they click? What feedback do they get?]

1. [Step 1]
2. [Step 2]
3. [Step 3]

### Layout & Visual Reasoning
[Where does this feature live in the interface? How does it relate to existing elements? What visual hierarchy applies?]

### Interaction Design
[How does the feature respond to user input? Transitions, loading states, error states, empty states.]

### Design Principles Applied
[Which principles guide the design — simplicity, progressive disclosure, consistency with existing UI, etc.]

### Accessibility Considerations
[Keyboard navigation, screen reader support, color contrast — anything relevant.]
```

If the feature is backend-only or CLI-only, adapt this section to describe the developer experience (DX) instead of visual UI — command structure, output format, error messages, discoverability.

## Step 8: Output the complete spec

Combine all sections into a single document with this structure:

```
# [Requirement Title]

## Business Requirements
...

## Strategic & Tactical Context
...

## User Stories
...

## High-Level Design
...

## Visual & UX Design
...
```

## Step 9: Cite journal sources

At the end of the spec, include a **Sources** section listing every journal page that informed the refinement:

```
## Sources

Journal pages consulted:
- decisions/[page] — [how it influenced this spec]
- architecture/[page] — [how it influenced this spec]
- learnings/[page] — [how it influenced this spec]
- users/[page] — [how it influenced this spec]
```

This skill is read-only. It does not create or modify journal pages. If the user wants to store the spec, they can do so separately.

## Reference

Detailed methodology and templates: `get_page` → `references/skills/refine-requirements/methodology.txt`
