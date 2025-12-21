package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/jehaj/new-year-wrapped/view"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("sqlite3", "./wrapped.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Initialize schema
	schema, err := os.ReadFile("db/schema.sql")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		log.Fatal(err)
	}

	e := echo.New()
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			fmt.Printf("REQUEST: uri: %v, status: %v\n", v.URI, v.Status)
			return nil
		},
	}))
	e.Use(middleware.Recover())

	e.GET("/party/:id/join", handleJoinPage)
	e.POST("/party/:id/join", handleJoin)
	e.GET("/search-songs", handleSongSearch)
	e.POST("/party/:id/submit-songs", handleSubmitSongs)
	e.GET("/create-party", handleCreateParty)

	e.Logger.Fatal(e.Start(":8080"))
}

func handleJoinPage(c echo.Context) error {
	partyID := c.Param("id")
	return view.JoinPage(partyID).Render(c.Request().Context(), c.Response().Writer)
}

func handleJoin(c echo.Context) error {
	// Handle user joining a party
	return nil
}

func handleSongSearch(c echo.Context) error {
	// HTMX sends the value of the input that triggered the request.
	// We need to find which song input it was.
	var query string
	var index int

	for i := 1; i <= 3; i++ {
		key := fmt.Sprintf("song-%d", i)
		if q := c.QueryParam(key); q != "" {
			query = q
			index = i
			break
		}
	}

	if query == "" {
		return c.HTML(http.StatusOK, "")
	}

	// Mock search results
	results := []view.SongResult{
		{ID: "yt-1", Title: query + " (Official Video)", Artist: "Popular Artist", Thumbnail: "https://via.placeholder.com/40"},
		{ID: "yt-2", Title: query + " (Live)", Artist: "Popular Artist", Thumbnail: "https://via.placeholder.com/40"},
		{ID: "yt-3", Title: query + " (Remix)", Artist: "DJ Remix", Thumbnail: "https://via.placeholder.com/40"},
	}

	c.Response().Header().Set("Content-Type", "text/html")
	return view.SearchResults(index, results).Render(c.Request().Context(), c.Response().Writer)
}
func handleSubmitSongs(c echo.Context) error {
	partyID := c.Param("id")
	name := c.FormValue("name")

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Create user
	res, err := tx.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, name)
	if err != nil {
		return err
	}
	userID, _ := res.LastInsertId()

	// Insert songs
	for i := 1; i <= 3; i++ {
		titleArtist := c.FormValue(fmt.Sprintf("song-%d", i))
		youtubeID := c.FormValue(fmt.Sprintf("song-id-%d", i))

		if titleArtist != "" && youtubeID != "" {
			// Simple split for title/artist if we have it, or just store as title
			// In a real app, we'd have separate fields from the search result
			_, err = tx.Exec("INSERT INTO songs (user_id, title, artist, youtube_id) VALUES (?, ?, ?, ?)",
				userID, titleArtist, "", youtubeID)
			if err != nil {
				return err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return c.HTML(http.StatusOK, "<div class='text-center'><h2 class='text-2xl font-bold'>Thanks!</h2><p>Your songs have been submitted.</p></div>")
}

func handleCreateParty(c echo.Context) error {
	id := "test-party"
	_, err := db.Exec("INSERT OR IGNORE INTO parties (id, name) VALUES (?, ?)", id, "Test Party")
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/party/"+id+"/join")
}
