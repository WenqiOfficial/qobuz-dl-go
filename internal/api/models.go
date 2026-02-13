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
	Title   string         `json:"title"`
	Version string         `json:"version"`
	Album   *AlbumMetadata `json:"album"`

	// Performer info
	Performer struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"performer"`
	Performers string `json:"performers"` // Full performers string with roles, e.g. "Artist, MainArtist - Composer, Composer"

	// Composer info (for classical music)
	Composer *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"composer"`
	Work string `json:"work"` // Classical work name

	// Track identifiers
	ID   int    `json:"id"`
	ISRC string `json:"isrc"` // International Standard Recording Code

	// Track position
	TrackNumber int `json:"track_number"`
	MediaNumber int `json:"media_number"`
	Duration    int `json:"duration"`

	// Audio quality info
	MaximumBitDepth     int     `json:"maximum_bit_depth"`
	MaximumSamplingRate float64 `json:"maximum_sampling_rate"`
	MaximumChannelCount int     `json:"maximum_channel_count"`
	HiresStreamable     bool    `json:"hires_streamable"`

	// ReplayGain info
	AudioInfo *struct {
		ReplayGainTrackGain float64 `json:"replaygain_track_gain"`
		ReplayGainTrackPeak float64 `json:"replaygain_track_peak"`
	} `json:"audio_info"`

	// Copyright and dates
	Copyright           string `json:"copyright"`
	ReleaseDateOriginal string `json:"release_date_original"`

	// Flags
	ParentalWarning bool `json:"parental_warning"`
}

// AlbumMetadata contains all metadata for an album.
type AlbumMetadata struct {
	// Basic info
	ID      string `json:"id"`
	Title   string `json:"title"`
	Version string `json:"version"`

	// Artist info
	Artist struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"artist"`
	Artists []struct {
		ID    int      `json:"id"`
		Name  string   `json:"name"`
		Roles []string `json:"roles"`
	} `json:"artists"`

	// Composer info (for classical)
	Composer *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"composer"`

	// Label info
	Label *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"label"`

	// Genre info
	Genre *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genre"`
	GenresList []string `json:"genres_list"` // Full genre list

	// Release info
	ReleaseDateOrg    string `json:"release_date_original"`
	ReleaseDateStream string `json:"release_date_stream"`
	ReleaseType       string `json:"release_type"` // album, single, ep, etc.

	// Tracks
	Tracks struct {
		Items []TrackMetadata `json:"items"`
	} `json:"tracks"`
	TracksCount int `json:"tracks_count"`
	MediaCount  int `json:"media_count"` // Number of discs

	// Cover art
	Image struct {
		Small     string `json:"small"`
		Large     string `json:"large"`
		Thumbnail string `json:"thumbnail"`
		Back      string `json:"back"`
	} `json:"image"`

	// Audio quality
	Duration            int     `json:"duration"`
	MaximumBitDepth     int     `json:"maximum_bit_depth"`
	MaximumSamplingRate float64 `json:"maximum_sampling_rate"`
	MaximumChannelCount int     `json:"maximum_channel_count"`
	HiresStreamable     bool    `json:"hires_streamable"`
	Streamable          bool    `json:"streamable"`

	// Copyright and identifiers
	Copyright string `json:"copyright"`
	UPC       string `json:"upc"` // Universal Product Code / Barcode

	// Additional info
	Description     string `json:"description"`
	Subtitle        string `json:"subtitle"`
	ParentalWarning bool   `json:"parental_warning"`
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
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Albums struct {
		Items []AlbumMetadata `json:"items"`
		Total int             `json:"total"`
	} `json:"albums"`
}
