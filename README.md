# Ralph

Ralph is a method. Ralph is not a specific model, assistant, prompt nor is it an exact spec template.

Ralph is continuously looping on a given prompt with the ability to pause/edit/resume, to iteratively build the application and the spec together.

## Ralph as a Tool

- Makes it easy to install and run the ralph loop
- A basic terminal UI that visualizes the live streamed output
- A default prompt for the loop itself

### Quickstart

1. Create a new repo.

2. Create specs/default.md, put 5-10 lines of a description of what you'd like built.

3. Run ralph: `ralph`

### Ralph is for Iterative Workflows

Ralph enables workflows where:
- You evolve a spec by sending it through Claude 10, 50, `n` times
- Each iteration informs the next.
- You can watch the evolution and catch regressions in real-time
- You can tune the specs by observing behavior, cancelling the loop, editing the spec, and resuming the loop.
- You can delete IMPLEMENTATION_PLAN.md at any point in time and restart the loop.

