# Ralph Tutorial: Ship Your First Spec-as-Code App

This guide walks you from zero to a functioning Ralph loop so you can author specs-as-code, watch the AI assistant execute them, and iterate quickly. You'll learn what the tool does, how to install it, how to prepare your repo, and how to guide the loop through its first real task.

## 1. What Ralph Does
Ralph is a simple loop: it keeps asking the AI assistant to work on your project, shows the conversation in a friendly terminal window, remembers how many tokens and dollars you have spent, and lets you pause whenever you want to make edits before resuming.

## 2. Prerequisites
- macOS or Linux shell with 256-color support.
- [Claude CLI](https://docs.anthropic.com/en/api/claude-cli) installed and logged in (`claude whoami` should work).
- Git.

## 3. Install Ralph
```bash
brew tap agentic-metallurgy/tap
brew install ralph
```

## 4. Prepare Your Repo
Run Ralph **inside** the repo that houses your code. Create a `specs` folder if it does not exist yet, and keep your spec files in there so Claude can find them.

## 5. Draft Your First Spec
Create `specs/default.md` with 5–10 punchy lines describing the product. Example:
```markdown
# default.md
- Build an MCP server using fastmcp that stores simple text notes and reminders.
- Notes support add, list, search, and delete actions.
- Reminders contain a message + due timestamp and can be created, listed, or removed.
- Persist all data to local JSON files so it survives restarts.
- Expose minimal MCP tools like add_note, list_notes, search_notes, add_reminder, list_reminders, etc.
- Provide a `docker compose up` entry point.
- Make the MCP interface available at http://localhost:7878/mcp.
```
Treat this file as the canonical truth. Because the default prompt explicitly tells Claude to "familiarize yourself with the specs in the specs/ directory," it will open this file at the start of every iteration.

## 6. Run the Loop
From the project root:
```bash
ralph --iterations 10      # default prompt, specs/ folder, 10 passes
```

Ralph streams Claude's work into the TUI immediately; you can quit early with `q` or `Ctrl+C`. Stats persist to `.ralph.claude_stats`, so the next run resumes the elapsed-time counter and usage totals.

## 7. Read the TUI
The interface has two major regions:
- **Status + Activity Feed**: The top panel shows `RUNNING` or `STOPPED`. Inside, every message line is tagged with an emoji (🤖 assistant, 🔧 tool use, 📝 user, 💰 system/cost, 💭 thinking, 🚀 loop marker, 🛑 stop, 💤 rate-limit hibernation). You can watch when Claude enters/exits loops, issues shell commands, or runs tests.
- **Footer**: Two side-by-side panels — "Usage & Cost" (tokens, per-direction counts, cumulative USD) on the left, "Loop Details" (progress e.g. 3/10, elapsed time) on the right. Below both panels sits a hotkey bar showing available controls.

## 8. Pause, Edit, Resume
1. Press `p` to pause the loop. Ralph cancels the in-flight iteration, swaps the status color to red, and freezes the timer.
2. Modify `specs/default.md`, edit or delete `IMPLEMENTATION_PLAN.md`, then save your changes.
3. Stage code changes manually if you want Claude to see a clean diff.
4. Press `r` or `s` to resume the loop. Ralph resumes from the same iteration number (it repeats the interrupted pass). You can also use `+`/`-` to adjust the remaining iteration count.

This workflow lets you steer Claude tightly: capture regressions, adjust specs, and rerun after manual fixes.

## 9. Validate and Commit
Ralph's default instructions already tell Claude to run tests and make commits for you. Expect local commits (but no pushes) to appear as the loop progresses. Keep any helper scripts (`make test`, etc.) ready so Claude can use them.

## 10. Recap Workflow
1. Install Ralph and Claude CLI.
2. Create `specs/default.md` (Ralph will create `IMPLEMENTATION_PLAN.md` the first time it runs, and you can edit it afterward).
3. Run `ralph --iterations N` from the repo root.
4. Watch the TUI, pausing whenever you want to edit specs or code.
5. Let Ralph run until a milestone lands, validate locally, and commit.

That's your first spec-as-code loop: write specs like regular code, keep them under version control, and let Ralph continuously translate them into working software.
