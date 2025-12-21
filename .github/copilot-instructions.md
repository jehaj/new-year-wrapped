# New Year Wrapped - AI Coding Instructions

This project is a "New Year Wrapped" game where friends compete to guess each other's music taste. It uses the **GoTH stack** (Go, Templ, HTMX) with **SQLite**.

## Architecture & Tech Stack
- **Backend**: Go (Standard library + HTMX-friendly handlers).
- **Frontend**: [Templ](https://templ.guide/) for type-safe HTML components and [HTMX](https://htmx.org/) for server-driven interactivity.
- **Database**: SQLite (local file-based storage).
- **Design Philosophy**: Server-side state management. Minimize client-side JavaScript; use HTMX for partial page updates and swaps.

## Critical Workflows
- **Templ Generation**: Always run `templ generate` after modifying `.templ` files to update the generated Go code.
- **Database**: Use migrations or a simple schema initialization script for SQLite.
- **Development**: Run the app using `go run main.go` (or the appropriate entry point in `cmd/`).

## Coding Conventions
- **HTMX Patterns**: 
    - Use `hx-target` and `hx-swap` to update specific UI sections (e.g., leaderboard updates, song reveals).
    - Return HTML fragments from Go handlers, not JSON.
- **Templ Components**:
    - Define reusable UI components in `.templ` files.
    - Pass data into components via Go structs.
- **State Management**: Keep the "source of truth" in the SQLite database. Use Go sessions or simple cookies for user identification.

## Key Components & Logic
- **Multi-party Support**: The system must handle multiple independent groups/parties.
- **Song Submission**: Each user submits exactly three songs.
- **Advent Theme**: Logic to restrict access to "doors" based on the current date.
- **Musical Imposter**: A scoring algorithm that tracks who received the most incorrect guesses.
- **Round-based Flow**: A state machine to transition between:
    1. Guessing (5 songs)
    2. Reveal (Song owners)
    3. Leaderboard update
- **Admin/TV View**: A specialized view optimized for large screens, showing real-time guess status and overall rankings.

## Reference Files
- `README.md`: Contains the initial feature list and stack description.
- `main.go`: (To be created) Entry point and route definitions.
- `view/`: (To be created) Directory for `.templ` files.
