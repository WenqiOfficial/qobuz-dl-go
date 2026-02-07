package api

// LoginResponse represents the response from the user/login endpoint.
type LoginResponse struct {
	UserAuthToken string `json:"user_auth_token"`
	User          struct {
		Email string `json:"email"`
		ID    int    `json:"id"`
	} `json:"user"`
}

// TrackURLResponse contains the download URL and format information for a track.
type TrackURLResponse struct {
	URL          string  `json:"url"`
	MimeType     string  `json:"mime_type"`
	SamplingRate float64 `json:"sampling_rate"`
	BitDepth     int     `json:"bit_depth"`
	Duration     int     `json:"duration"`
}

// TrackMetadata contains all metadata for a single track.
type TrackMetadata struct {
	Title     string         `json:"title"`
	Version   string         `json:"version"`
	Album     *AlbumMetadata `json:"album"`
	Performer struct {
		Name string `json:"name"`
	} `json:"performer"`
	MaximumSamplingRate float64 `json:"maximum_sampling_rate"`
	ID                  int     `json:"id"`
	Duration            int     `json:"duration"`
	TrackNumber         int     `json:"track_number"`
	MediaNumber         int     `json:"media_number"`
	MaximumBitDepth     int     `json:"maximum_bit_depth"`
	HiresStreamable     bool    `json:"hires_streamable"`
}

// AlbumMetadata contains all metadata for an album.
type AlbumMetadata struct {
	Genre *struct {
		Name string `json:"name"`
	} `json:"genre"`
	ID                string `json:"id"`
	Title             string `json:"title"`
	Version           string `json:"version"`
	ReleaseDateOrg    string `json:"release_date_original"`
	ReleaseDateStream string `json:"release_date_stream"`
	Artist            struct {
		Name string `json:"name"`
	} `json:"artist"`
	Tracks struct {
		Items []TrackMetadata `json:"items"`
	} `json:"tracks"`
	Image struct {
		Small string `json:"small"`
		Large string `json:"large"`
	} `json:"image"`
	Duration            int     `json:"duration"`
	TracksCount         int     `json:"tracks_count"`
	MaximumBitDepth     int     `json:"maximum_bit_depth"`
	MaximumSamplingRate float64 `json:"maximum_sampling_rate"`
	HiresStreamable     bool    `json:"hires_streamable"`
	Streamable          bool    `json:"streamable"`
	ReleaseType         string  `json:"release_type"`
}

// SearchAlbumsResponse is the response from album/search endpoint.
type SearchAlbumsResponse struct {
	Albums struct {
		Items []AlbumMetadata `json:"items"`
		Total int             `json:"total"`
		Limit int             `json:"limit"`
	} `json:"albums"`
}

// SearchTracksResponse is the response from track/search endpoint.
type SearchTracksResponse struct {
	Tracks struct {
		Items []TrackMetadata `json:"items"`
		Total int             `json:"total"`
		Limit int             `json:"limit"`
	} `json:"tracks"`
}

// ArtistMetadata contains metadata for an artist.
type ArtistMetadata struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	AlbumsCount int    `json:"albums_count"`
	Image       *struct {
		Small string `json:"small"`
		Large string `json:"large"`
	} `json:"image"`
}

// SearchArtistsResponse is the response from artist/search endpoint.
type SearchArtistsResponse struct {
	Artists struct {
		Items []ArtistMetadata `json:"items"`
		Total int              `json:"total"`
		Limit int              `json:"limit"`
	} `json:"artists"`
}

// ArtistAlbumsResponse is the response from artist/get with extra=albums.
type ArtistAlbumsResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Albums struct {
		Items []AlbumMetadata `json:"items"`
		Total int             `json:"total"`
	} `json:"albums"`
}
