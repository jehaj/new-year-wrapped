# New Year Wrapped

A Spotify Wrapped-themed competition website for friends. Join a party, submit your top songs of the year, and guess which songs belong to whom!

## Features

- **Party Management**: Create private parties with unique 6-character IDs.
- **Admin Security**: Secure admin actions (Start, Next Round) using a 12-character `admin_token`.
- **Song Submission**: Users join with their Name and Top 3 Songs.
- **Competition Logic**:
    - Shuffled song order across all participants.
    - Round-based gameplay (default 5 songs per round).
    - One-time guessing per song with disabled inputs after submission.
- **Leaderboards**:
    - **Round Results**: See who got points in the last revealed round.
    - **Global Leaderboard**: Track the overall winner across the entire party.
- **SSR Architecture**: Fast, server-side rendered UI using Go templates and Pico CSS.
- **Local Assets**: No external CDNs or Tailwind dependencies; everything is served locally.

## Tech Stack

- **Backend**: Go (Golang)
- **Database**: SQLite
- **Frontend**: Go `html/template`
- **Styling**: [Pico CSS (v2)](https://picocss.com/)
- **Logging**: Service-level logging to `party.log`

## Getting Started

### Prerequisites

- [Go](https://golang.org/doc/install) (only tested 1.25).

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/jehaj/new-year-wrapped.git
   cd new-year-wrapped
   ```

2. The project uses local CSS. Ensure `static/css/pico.min.css` is present.
   e.g. with
   ```bash
   mkdir -p static/css && curl -L https://cdn.jsdelivr.net/npm/
@picocss/pico@2/css/pico.min.css -o static/css/pico.min.css
   ```

### Running the Application

Start the server:
```bash
go run cmd/server/main.go
```
The server will start on `http://localhost:8080`.

### Running Tests

The project follows TDD principles. Run the test suite with:
```bash
go test ./...
```

## How to Play

1. **Create a Party**: One person creates a party and becomes the Admin. They receive a unique URL with an `admin_token`.
2. **Invite Friends**: Share the Party ID with your friends.
3. **Join**: Everyone joins by entering their name and their top 3 songs of the year.
4. **Start**: Once everyone has joined, the Admin starts the competition.
5. **Guess**: In each round, listen to/read the song titles and guess which friend they belong to.
6. **Reveal**: After each round, the Admin moves to the next round to reveal the correct owners and update the leaderboard.

## Project Structure

- `cmd/server/`: Application entry point and route registration.
- `internal/party/`: Core business logic, HTTP handlers, and service layer.
- `internal/db/`: SQLite database initialization and schema management.
- `templates/`: Server-side HTML templates (`layout.html`).
- `static/`: Static assets (CSS, images).
