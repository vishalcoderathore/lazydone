# lazydone

A keyboard-driven personal task manager for the terminal. One person's Tasks sit on a three-Status kanban Board, which the TUI and the command line both work on.

## Tasks

**Task**:
One thing the user means to do, with its text and optional Priority, Due date, Tags and Notes.
_Avoid_: Todo, item, card, entry

**Notes**:
Longer free text attached to a Task, separate from its one-line text.
_Avoid_: Description, body, details

**Priority**:
How important a Task is: none, low, medium or high.
_Avoid_: Importance, urgency, level

**Due date**:
The calendar day a Task should be finished by. It stays the same day wherever the user travels.
_Avoid_: Deadline, due time

**Overdue**:
A Task not yet Completed whose Due date is before today.
_Avoid_: Late, past due

**Tag**:
A word on a Task that groups it with related Tasks. A `+project` Tag says what the Task belongs to, and an `@context` Tag says where or with what it can be done.
_Avoid_: Label, category; "project" or "context" as terms of their own

## Status

**Status**:
Where a Task stands in its life: To Do, In Progress or Completed. Every Task has exactly one.
_Avoid_: Column (the Board's picture of a Status, not a term of its own), state, stage

**To Do**:
The Status of a Task not yet started. New Tasks begin here.
_Avoid_: Backlog, open, todo (except as the command-line short form)

**In Progress**:
The Status of a Task being worked on.
_Avoid_: Doing (except as the command-line short form), active, started

**Completed**:
The Status of a finished Task. It records the moment the Task was finished.
_Avoid_: Done (except as the command-line short form), closed, finished

**Archive**:
Where Completed Tasks are put away once they no longer need to be on the Board. An archived Task can be restored.
_Avoid_: History, trash, hidden

**Delete**:
Removing a Task for good, because it was a mistake. Archiving is for finished work instead.
_Avoid_: Remove, discard

## Looking at Tasks

**Board**:
All of the user's Tasks that are not archived, laid out by Status.
_Avoid_: List, kanban, todos

**View**:
Which set of Tasks is being shown: the Board or the Archive, optionally narrowed to Today.
_Avoid_: Mode, screen, page

**Today**:
A View of the Tasks that matter today: those due today or Overdue, plus those Completed today.
_Avoid_: Agenda, daily view, due list

**Filter**:
Showing only the Tasks that carry a chosen Tag.
_Avoid_: Tag view, project view

**Sort**:
Showing each Status's Tasks in order of Priority, Due date or creation, without changing the Manual order.
_Avoid_: Order by, ranking

**Manual order**:
The order the user has arranged Tasks in within a Status, kept no matter how they are sorted.
_Avoid_: Default order, position, rank

**Search**:
Moving between Tasks whose text, Tags or Notes loosely match what was typed. It hides nothing.
_Avoid_: Find, filter (Filter is by Tag)

## Getting Tasks in

**Capture**:
Writing a Task as one line of text, with any Tags and Due date inline, such as `fix bug +work due:fri`. It works the same in the TUI and on the command line.
_Avoid_: Quick add, entry, input

**Undo**:
Reversing the most recent change made in this session, without disturbing Tasks changed elsewhere.
_Avoid_: Revert, rollback
