package dto

type ContentSearchRequest struct {
	Query string `form:"q" binding:"required"`
}

type ContentResponse struct {
	BaseDTO
	ContentType string `json:"contentType"`
	Title       string `json:"title"`
	ReleaseYear *int   `json:"releaseYear,omitempty"`
	PosterUrl   string `json:"posterUrl,omitempty"`
}
