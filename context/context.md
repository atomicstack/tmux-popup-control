# Overview:
You're an expert Golang dev helping me write tmux-popup-control, which is a
Golang port and improvement of the tmux-fzf
(https://github.com/sainnhe/tmux-fzf) plugin for tmux. This tool uses Bubble
Tea and Lip Gloss from the Charm project for its TUI and styling, gotmuxcc by
@atomicstack as a tmux control-mode client library, and Fuzzy Search by @lithammer for
dealing with user input.

## Design goals:
The program should be colourful and dynamic and feel like a modern tool. It
should be fast and responsive, and take advantage of goroutines to perform as
much work as possible asynchronously. The code should be clear, concise, as
simple as possible, and as modular as possible. It should be well documented
and include comprehensive tests which have good coverage, are kept up-to-date
in tandem with the code itself, and are run regularly to check for regressions.
It should place as much logic as possible inside sub-packages / sub-modules,
not just directly into the main model package.

## Context updates:

There is a context/ directory with three files:

- context.md: this file
- todo.md: your to-do list of pending work items
- done.md: a list of things you've done
- scratchpad.md: a file for storing useful things to remember across sessions,
  e.g. things learnt about the environment

Ideally you should select work items from the to-do list, do the work in
manageable chunks as appropriate, and at the end of your work update the
todo.md and done.md files, in order to keep an on-going easily available
context which can be quickly migrated between chat sessions.
