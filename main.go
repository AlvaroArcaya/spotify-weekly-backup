package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/joho/godotenv/autoload"

	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

const redirectURI = "http://localhost:8080/callback"
const tokenFile = "token.data"
const discoverPlaylistName = "Discover Weekly"

var (
	auth   = spotify.NewAuthenticator(redirectURI, spotify.ScopeUserReadPrivate, spotify.ScopePlaylistModifyPrivate)
	ch     = make(chan *spotify.Client)
	state  = "abc123"
	client spotify.Client
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("No .env file found")
	}
}

func main() {
	if !fileExists(tokenFile) {
		// start an HTTP server
		http.HandleFunc("/callback", completeAuth)

		// listen server
		go http.ListenAndServe(":8080", nil)

		// generate authentication url
		url := auth.AuthURL(state)
		fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)

		// wait for auth to complete
		newClient := <-ch

		// set global client
		client = *newClient
	} else {
		// read previous token
		bytes, err := ioutil.ReadFile(tokenFile)
		if err != nil {
			log.Fatal(err)
		}
		// create a empty token
		tok := new(oauth2.Token)

		// change token to previous
		if err := json.Unmarshal(bytes, tok); err != nil {
			log.Fatalf("could not unmarshal token: %v", err)
		}

		// create client
		client = spotify.NewAuthenticator("").NewClient(tok)

		// renew the token
		newToken, err := client.Token()
		if err != nil {
			log.Fatalf("could not retrieve token from client: %v", err)
		}

		// update if the previous token is outdated
		if newToken.AccessToken != tok.AccessToken {
			log.Println("got refreshed token, saving it")

			data, _ := json.Marshal(newToken)
			ioutil.WriteFile(tokenFile, data, os.ModePerm)
		}
	}

	// get discover weekly playlist
	getPlaylist(client)
}

func getPlaylist(client spotify.Client) {
	// get current user
	user, err := client.CurrentUser()
	if err != nil {
		log.Fatal(err)
	}

	// Search for discover playlist
	result, err := client.Search(discoverPlaylistName, spotify.SearchTypePlaylist)
	if err != nil {
		log.Fatalf("could not get playlists: %v", err)
	}
	// Get the ID of the Discover Weekly playlist
	discoverID := result.Playlists.Playlists[0].ID

	// Get playlist by id
	discoverPlaylist, err := client.GetPlaylist(discoverID)
	if err != nil {
		log.Fatalf("could not get Discover Weekly playlist: %v", err)
	}

	// Generate track ids array
	trackIDs := make([]spotify.ID, 0, len(discoverPlaylist.Tracks.Tracks))

	// Extract the Track IDs for each song.
	for _, t := range discoverPlaylist.Tracks.Tracks {
		trackIDs = append(trackIDs, t.Track.SimpleTrack.ID)
	}

	// get current year and number week
	year, week := getWeekNumber()

	// Playlist name
	playlistName := fmt.Sprintf("Backup year: %d week: %d", year, week)

	// Create playlist
	newPlaylist, err := client.CreatePlaylistForUser(user.ID, playlistName, "", false)
	if err != nil {
		log.Fatalf("could not create playlist: %v", err)
	}

	// Add track ids array to new playlist
	client.AddTracksToPlaylist(newPlaylist.ID, trackIDs...)

}

func fileExists(tokenFile string) bool {
	info, err := os.Stat(tokenFile)
	if os.IsNotExist(err) {
		return false
	}

	return !info.IsDir()
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	// create a token
	tok, err := auth.Token(state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}

	// Save token
	data, _ := json.Marshal(tok)
	ioutil.WriteFile(tokenFile, data, os.ModePerm)

	// use the token to get an authenticated client
	client := auth.NewClient(tok)
	fmt.Fprintf(w, "Login Completed!")

	// notify to main
	ch <- &client
}

func getWeekNumber() (int, int) {
	tn := time.Now().UTC()
	year, week := tn.ISOWeek()

	return year, week
}
