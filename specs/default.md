- current ralph has an integrated prompt.md.
- current ralph only/directly builds.
- we want to change the default build prompt.md that ships with it, new build prompt: @PROMPT_build.md
- we want to add a 'plan' mode to it that uses new plan prompt: @PROMPT_build.md only triggered with `ralph plan`
- add a `ralph --version` to show version, too
- move the 'quit stop start' to be left aligned, light up the "quit" option, just like we light up start, when we stop (even though it's available during running, too)
- edit the bottom statusbar (maintain color) to show:

```
[current loop: #3/5      tokens: xxxxxx      elapsed: 33:99:00]
```

- and change the "Loop Details" box to be "Ralph Details":
  - Loop: #3/5
  - Total Time: hh:mm:ss
  - Status: Stopped/Running
  - Agents: <# of agents>
  - Task: ...

- and make "Total Tokens" under Usage & Cost to be human readable e.g. 36.87m or 300k
- Task: should remove "Task" from the title of the task it's showing so it doesn't look like `Task: Task: 6`, if the first word is in fact Task, it should look like `Task: 6 ...`
