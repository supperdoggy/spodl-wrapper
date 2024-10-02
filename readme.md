# Spotify Link Downloader Service

## Overview

This is a Golang service that retrieves Spotify playlist, album, and song links from a MongoDB database and downloads them using the `spotdl` utility. The service pulls the links from MongoDB, invokes `spotdl` to download the corresponding media, and manages the download process efficiently.

## Features

- Fetches Spotify links (playlists, albums, songs) from MongoDB.
- Downloads tracks using the `spotdl` command-line utility.
- Supports queue management for batch downloading.
- Logs download statuses and errors for easy troubleshooting.

## Prerequisites

- **Golang** 1.21 or higher
- **spotdl** utility installed (follow [spotdl documentation](https://github.com/spotDL/spotify-downloader))
- **MongoDB** for storing Spotify links
- **ffmpeg** for audio processing (required by `spotdl`)
- Environment configured for MongoDB access

## Installation

1. **Clone the repository**:

    ```bash
    git clone https://github.com/yourusername/spotify-link-downloader.git
    cd spotify-link-downloader
    ```

2. **Install required dependencies**:

    ```bash
    go mod tidy
    ```

3. **Install `spotdl`**:

    Follow the official installation guide to install `spotdl` and its dependencies:

    ```bash
    pip install spotdl
    ```

4. **Set up MongoDB**:

    Make sure you have a MongoDB instance running. You can either use a local MongoDB instance or a managed service like MongoDB Atlas.

    Example MongoDB document structure for Spotify links:
    
    ```json
    {
      "_id": ObjectId("64d4c75b0000000000000000"),
      "spotify_url": "https://open.spotify.com/track/1234567890abcdefghij",
      "status": "pending"  // Status could be 'pending', 'downloading', 'completed', 'failed'
    }
    ```

5. **Set environment variables**:

    You can store sensitive information like MongoDB connection strings in a `.env` file.

    ```env
    MONGO_URI=mongodb://localhost:27017
    MONGO_DB=spotify_links_db
    MONGO_COLLECTION=links
    DOWNLOAD_DIR=/path/to/downloads
    ```

6. **Run the service**:

    ```bash
    go run main.go
    ```

## Configuration

Ensure you have configured `spotdl` properly, including setting the location for downloaded files. The service will use this location to save the downloaded Spotify tracks.

## Usage

1. The service will automatically pull Spotify links from the MongoDB collection that have a `status` of `pending`.
2. For each link, the service will invoke `spotdl` to download the track, playlist, or album to the specified directory.
3. Once the download is complete, the status of the link will be updated in MongoDB (e.g., `completed`, `failed`).

## Example Commands

- Download a Spotify song:

    ```bash
    spotdl https://open.spotify.com/track/1234567890abcdefghij
    ```

- Download a Spotify album:

    ```bash
    spotdl https://open.spotify.com/album/abcdefghij1234567890
    ```

- Download a Spotify playlist:

    ```bash
    spotdl https://open.spotify.com/playlist/abcdefghij0987654321
    ```

## MongoDB Integration

The service interacts with a MongoDB collection to store Spotify links and track their download status. The links are retrieved periodically or in batch from the MongoDB collection, and their status is updated after each download attempt.

- **Pending**: The initial state when the link is added.
- **Downloading**: When the service has started the download process for a link.
- **Completed**: When the link has been successfully downloaded.
- **Failed**: If the download fails for any reason.

## Logging

Logs are generated for each link processed, detailing whether the download was successful or if there were errors. This can help with debugging and monitoring download progress.

## Contributing

1. Fork the repository.
2. Create your feature branch: `git checkout -b feature/my-new-feature`
3. Commit your changes: `git commit -am 'Add some feature'`
4. Push to the branch: `git push origin feature/my-new-feature`
5. Submit a pull request!

## License

This project is licensed under the MIT License.
