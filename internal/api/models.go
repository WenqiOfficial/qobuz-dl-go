package api

type LoginResponse struct {
	UserAuthToken string `json:"user_auth_token"`
	User          struct {
		ID    int    `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

type TrackURLResponse struct {
	URL          string  `json:"url"`
	BitDepth     int     `json:"bit_depth"`
	SamplingRate float64 `json:"sampling_rate"`
	MimeType     string  `json:"mime_type"`
	Duration     int     `json:"duration"`
}

type TrackMetadata struct {
	ID                  int    `json:"id"`
	Title               string `json:"title"`
	Version             string `json:"version"`
	Duration            int    `json:"duration"`
	TrackNumber         int    `json:"track_number"`
	MediaNumber         int    `json:"media_number"`
	MaximumBitDepth     int    `json:"maximum_bit_depth"`
	MaximumSamplingRate float64 `json:"maximum_sampling_rate"`
	Performer           struct {
		Name string `json:"name"`
	} `json:"performer"`
	Album struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Artist struct {
			Name string `json:"name"`
		} `json:"artist"`
	} `json:"album"`
}
