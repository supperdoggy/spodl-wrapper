package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/zmb3/spotify/v2"
)

type PlaylistTrack struct {
	Name          string   `json:"name"`
	Artists       []string `json:"artists"`
	Artist        string   `json:"artist"`
	Genres        []string `json:"genres"`
	DiscNumber    int      `json:"disc_number"`
	DiscCount     int      `json:"disc_count"`
	AlbumName     string   `json:"album_name"`
	AlbumArtist   string   `json:"album_artist"`
	Duration      int      `json:"duration"`
	Year          string   `json:"year"`
	Date          string   `json:"date"`
	TrackNumber   int      `json:"track_number"`
	TracksCount   int      `json:"tracks_count"`
	SongID        string   `json:"song_id"`
	Explicit      bool     `json:"explicit"`
	Publisher     string   `json:"publisher"`
	URL           string   `json:"url"`
	ISRC          string   `json:"isrc"`
	CoverURL      string   `json:"cover_url"`
	CopyrightText string   `json:"copyright_text"`
	DownloadURL   *string  `json:"download_url"` // Nullable field
	Lyrics        string   `json:"lyrics"`
	Popularity    int      `json:"popularity"`
	AlbumID       string   `json:"album_id"`
	ListName      string   `json:"list_name"`
	ListURL       string   `json:"list_url"`
	ListPosition  int      `json:"list_position"`
	ListLength    int      `json:"list_length"`
	ArtistID      string   `json:"artist_id"`
	AlbumType     string   `json:"album_type"`
}

func GetPlaylistData(playlistURL string, spotifyClient *spotify.Client) (string, []string, error) {
	// Create a temporary file to save the playlist with spotdl
	tempFile, err := ioutil.TempFile("", "playlist-*.spotdl")
	if err != nil {
		fmt.Println("Error creating temporary file:", err)
		return "", nil, err
	}
	defer os.Remove(tempFile.Name()) // Clean up temporary file after execution

	// TODO migrate fully to spotify api and remove spotdl here

	// Run spotdl to save the playlist to the temporary file
	cmd := exec.Command("spotdl", "save", playlistURL, "--save-file", tempFile.Name())
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error running spotdl:", err)
		return "", nil, err
	}

	// Read the JSON data from the saved file
	file, err := ioutil.ReadFile(tempFile.Name())
	if err != nil {
		fmt.Println("Error reading file:", err)
		return "", nil, err
	}

	var songs []PlaylistTrack
	err = json.Unmarshal(file, &songs)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return "", nil, err
	}

	// Extract the list name from the playlist URL

	playlistID := strings.Split(strings.Split(playlistURL, "/")[4], "?")[0]
	playlist, err := spotifyClient.GetPlaylist(context.Background(), spotify.ID(playlistID))
	if err != nil {
		fmt.Println("Error getting playlist:", err)
		return "", nil, err
	}

	playlistName := playlist.Name

	// Create a list of "Artist - Song" strings
	var songList []string
	for _, song := range songs {
		artist := strings.Join(song.Artists, ", ")
		if artist == "" {
			artist = song.Artist
		}
		songList = append(songList, fmt.Sprintf("%s - %s", artist, song.Name))
	}

	return playlistName, songList, nil
}

// CreateM3UPlaylist generates an .m3u playlist from a list of "Artist - Song" strings
func CreateM3UPlaylist(songList []string, musicRoot, outputPath string) error {
	var matchedPaths []string

	matchedSongs := make(map[string]bool)

	err := filepath.Walk(musicRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		lowerPath := strings.ToLower(path)
		for _, song := range songList {
			parts := strings.SplitN(song, " - ", 2)
			if len(parts) != 2 {
				continue
			}

			artist := strings.ToLower(strings.TrimSpace(parts[0]))
			title := strings.ToLower(strings.TrimSpace(parts[1]))

			if !strings.HasSuffix(lowerPath, ".lrc") {
				if _, ok := matchedSongs[song]; ok {
					continue // Skip if already matched
				}

				if strings.Contains(lowerPath, strings.ToLower(song)+".") {
					relPath, _ := filepath.Rel(filepath.Dir(outputPath), path)

					// TODO FIX IT LATER
					relPath = strings.Replace(relPath, "../../", "", 1)
					relPath = strings.ReplaceAll(relPath, "..", "Job-downloaded")
					relPath = "/music/" + relPath
					matchedPaths = append(matchedPaths, relPath)
					matchedSongs[song] = true
					break // prevent duplicate matches
				} else if strings.Contains(lowerPath, artist) && strings.Contains(lowerPath, title) {
					relPath, _ := filepath.Rel(filepath.Dir(outputPath), path)

					// TODO FIX IT LATER
					relPath = strings.Replace(relPath, "../../", "", 1)
					relPath = strings.ReplaceAll(relPath, "..", "Job-downloaded")
					relPath = "/music/" + relPath
					matchedPaths = append(matchedPaths, relPath)
					matchedSongs[song] = true
					break // prevent duplicate matches
				}
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	// check if file already exists
	if _, err := os.Stat(outputPath); err == nil {
		return os.ErrExist
	}

	// Write the M3U file
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, path := range matchedPaths {
		_, err := f.WriteString(path + "\n")
		if err != nil {
			return err
		}
	}

	return nil
}
