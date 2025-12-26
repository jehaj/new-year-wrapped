# New Year Wrapped - AI Coding Instructions

## Project Overview
A Spotify Wrapped-themed competition website using YouTube Music data. Friends join a "Party" and guess which top songs belong to whom.

## Tech Stack & Architecture
- **Backend**: Go (Golang). Use the inbuilt router or Chi/Gin/Mux, preferring simplicity.
- **Database**: SQLite.
- **Frontend Styling**: Pico CSS (v2). 
    - **Constraint**: Do NOT use Tailwind or CDNs. 
    - **Delivery**: Serve Pico CSS locally from `static/`
- **Architecture**: Simple monolithic structure. Focus on clear service boundaries for Party Management and Competition Logic.

## Core Logic & Flow
- **Parties**: Create (Admin) -> Join (Users provide Name + Top 3 Songs) -> Waiting.
- **Competition**: Admin starts -> Shuffled songs -> N rounds (5 songs each).
- **Guessing**: Advent calendar UI. Searchable dropdown for owner guesses.
- **Leaderboards**: Global and Round-specific updates after each reveal.

## Developer Workflows
- **TDD (Test Driven Development)**: Always write tests for database operations and route handlers *before* implementation.
- **Testing Database**: Use `:memory:` for SQLite tests to ensure isolation and speed.
- **Commands**:
    - Run tests: `go test ./...`
    - Run app: `go run cmd/server/main.go`

## Coding Conventions
- **Simplicity First**: Avoid over-engineering. Prefer standard library where possible.
- **Routing**: Use Go 1.22+ `http.ServeMux` with path parameters (e.g., `r.PathValue("id")`).
- **Database**: Use clean SQL queries or a simple wrapper.
- **UI**: Maintain the "Spotify Wrapped" aesthetic using Pico CSS components.

## Key Files & Directories
- `static/`: Local assets including Pico CSS.
- `internal/party/`: Party management logic and handlers.
- `internal/db/`: Database initialization and schema.
- `cmd/server/main.go`: Application entry point.
