# Todo List App - Implementation Plan

A simple CLI todo app based on bash (todo.sh) that saves todos in a text file.

## Specifications

Based on `specs/default.md`:
1. CLI todo app based on bash
2. Script name: `todo.sh`
3. Saves todos in a text file
4. Only supports adding todos: `./todo.sh add 'do laundry'`
5. Todos are stored one per line

---

## TASKS (Prioritized)

### TASK 1: Create Basic todo.sh Script [HIGH PRIORITY]
**Status**: COMPLETED

Create the bash script with basic structure and add command.

**Steps**:
1. Create `todo.sh` bash script with shebang
2. Make it executable
3. Parse command-line arguments
4. Implement `add` command that takes a quoted argument
5. Display usage/help if no valid command provided

**Validation**:
- [x] Script is executable (`chmod +x todo.sh`)
- [x] `./todo.sh` without args shows usage
- [x] `./todo.sh add 'task'` adds a task

---

### TASK 2: Text File Persistence [HIGH PRIORITY]
**Status**: COMPLETED

Store todos in a text file, one per line.

**Steps**:
1. Define todo file location (e.g., `todos.txt` in same directory)
2. Append new todos to file
3. Ensure newline after each todo
4. Create file if it doesn't exist

**Validation**:
- [x] `./todo.sh add 'first task'` creates todos.txt
- [x] `./todo.sh add 'second task'` appends correctly
- [x] Each todo is on its own line
- [x] No duplicate newlines

---

### TASK 3: Testing and Validation [MEDIUM PRIORITY]
**Status**: COMPLETED

Test all functionality.

**Steps**:
1. Test adding single todo
2. Test adding multiple todos
3. Verify file format (one per line)
4. Test edge cases (empty todo, special characters)

**Validation**:
- [x] All basic operations work
- [x] File format is correct
- [x] Script handles edge cases gracefully

---

## Progress Log

| Date | Task | Status | Notes |
|------|------|--------|-------|
| 2025-12-03 | TASK 1: Create Basic todo.sh Script | COMPLETED | Created executable bash script with add command, usage display |
| 2025-12-03 | TASK 2: Text File Persistence | COMPLETED | Todos saved to todos.txt, one per line, handles file creation |
| 2025-12-03 | TASK 3: Testing and Validation | COMPLETED | Tested adding todos, multiple entries, edge cases (empty, special chars) |

---

## Architecture Notes

### Simple Data Flow
```
./todo.sh add 'task'
    ↓
Parse arguments (command = 'add', task = 'task')
    ↓
Append task to todos.txt with newline
    ↓
Done
```

### File Format
```
todos.txt:
do laundry
buy groceries
finish report
```
