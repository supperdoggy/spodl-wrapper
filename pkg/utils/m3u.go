package utils

import (
	"context"
	"fmt"
	"os"
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

	// Extract the list name from the playlist URL

	playlistID := strings.Split(strings.Split(playlistURL, "/")[4], "?")[0]
	playlist, err := spotifyClient.GetPlaylist(context.Background(), spotify.ID(playlistID))
	if err != nil {
		fmt.Println("Error getting playlist:", err)
		return "", nil, err
	}

	playlistName := playlist.Name

	var playlistItems []spotify.PlaylistItem
	itemsPage, err := spotifyClient.GetPlaylistItems(context.Background(), spotify.ID(playlistID))
	if err != nil {
		fmt.Println("Error getting playlist items:", err)
		return "", nil, err
	}

	if itemsPage.Total > spotify.Numeric(len(itemsPage.Items)) {
		total := int(itemsPage.Total)
		for i := 0; i < total; i += int(itemsPage.Limit) {
			items, err := spotifyClient.GetPlaylistItems(context.Background(), spotify.ID(playlistID), spotify.Limit(int(itemsPage.Limit)), spotify.Offset(i))
			if err != nil {
				fmt.Println("Error getting playlist items:", err)
				return "", nil, err
			}
			playlistItems = append(playlistItems, items.Items...)
		}
	} else {
		playlistItems = itemsPage.Items
	}

	// Create a list of "Artist - Song" strings
	var songList []string
	for _, song := range playlistItems {
		artistNames := make([]string, len(song.Track.Track.Artists))
		for i, artist := range song.Track.Track.Artists {
			artistNames[i] = artist.Name
		}
		songList = append(songList, fmt.Sprintf("%s - %s", strings.Join(artistNames, ", "), song.Track.Track.Name))
	}

	return playlistName, songList, nil
}

// CreateM3UPlaylist generates an .m3u playlist from a list of "Artist - Song" strings
func CreateM3UPlaylist(songList []string, musicRoot, outputPath string) error {
	var matchedPaths []string
	matchedSongs := make(map[string]bool)

	// check if file already exists
	if _, err := os.Stat(outputPath); err == nil {
		return os.ErrExist
	}

	for _, song := range songList {
		parts := strings.SplitN(song, " - ", 2)
		if len(parts) != 2 {
			continue
		}

		// artist := strings.ToLower(strings.TrimSpace(parts[0]))
		// title := strings.ToLower(strings.TrimSpace(parts[1]))
		songLower := strings.ToLower(song)

		var matchedPath string

		filepath.Walk(musicRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || strings.HasSuffix(strings.ToLower(path), ".lrc") {
				return nil
			}

			if _, ok := matchedSongs[song]; ok {
				return filepath.SkipDir // skip remaining walk if already matched
			}

			lowerPath := strings.ToLower(path)

			if strings.Contains(lowerPath, songLower+".") { // || (strings.Contains(lowerPath, artist) && strings.Contains(lowerPath, title))

				relPath, _ := filepath.Rel(filepath.Dir(outputPath), path)
				relPath = strings.Replace(relPath, "../../", "", 1)
				relPath = strings.ReplaceAll(relPath, "..", "Job-downloaded")
				relPath = "/music/" + relPath

				matchedPath = relPath
				matchedSongs[song] = true
				return filepath.SkipDir // stop walking further for this song
			}

			return nil
		})

		if matchedPath != "" {
			matchedPaths = append(matchedPaths, matchedPath)
		}
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
