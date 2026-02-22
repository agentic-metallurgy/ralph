0a. familiarize yourself with the source code in this directory, use up to 50 parallel Sonnet subagents.
0b. familiarize yourself with the specs in the specs/ directory

1. read @IMPLEMENTATION_PLAN.md and implement the single highest priority TASK using up to 5 Opus subagents
2. Ensure all tests and linting passes, then update IMPLEMENTATION_PLAN.md with your progress
3. Ensure implementation steps are organized around verifiable milestones, and that you have either a) validated them or b) documented the validation steps or what's not working.
4. use `git add -A` and `git commit -m "..."` to commit your changes - do not include any claude attribution
5. If the implemented features don't match IMPLEMENTATION_PLAN.md, correct the plan.

99. You may use up to 10 parallel Sonnet subagents for searches/reads, and only 1 Opus subagent for build/tests.
999. Single sources of truth, no migrations/adapters. If tests unrelated to your work fail, resolve them as part of the increment.
9999. You may add extra logging if required to debug issues.
99999. Keep @IMPLEMENTATION_PLAN.md current with learnings using a subagent — future work depends on this to avoid duplicating efforts. Update especially after finishing your turn.
999999. When you learn something new about how to run the application, update @AGENTS.md using a subagent but keep it brief. For example if you run commands multiple times before learning the correct command then that file should be updated.
9999999. For any bugs you notice, resolve them or document them in @IMPLEMENTATION_PLAN.md using a subagent even if it is unrelated to the current piece of work.
99999999. Implement functionality completely. Placeholders and stubs waste efforts and time redoing the same work.
999999999. IMPORTANT: Keep @AGENTS.md operational only — status updates and progress notes belong in `IMPLEMENTATION_PLAN.md`. A bloated AGENTS.md pollutes every future loop's context.
