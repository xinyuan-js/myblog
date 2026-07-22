package blog

import "time"

type SocialLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
	Icon  string `json:"icon"`
}

type SiteProfile struct {
	Title       string       `json:"title"`
	Subtitle    string       `json:"subtitle"`
	Description string       `json:"description"`
	AvatarURL   *string      `json:"avatarUrl"`
	BannerURL   *string      `json:"bannerUrl"`
	AuthorName  string       `json:"authorName"`
	AuthorBio   string       `json:"authorBio"`
	SocialLinks []SocialLink `json:"socialLinks"`
	ICPNumber   *string      `json:"icpNumber"`
}

type Tag struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	PostCount int64  `json:"postCount"`
}

type Category struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description"`
	PostCount   int64   `json:"postCount"`
}

type PostLink struct {
	Title string `json:"title"`
	Slug  string `json:"slug"`
}

type PostSummary struct {
	ID                 int64      `json:"id"`
	Title              string     `json:"title"`
	Slug               string     `json:"slug"`
	Excerpt            string     `json:"excerpt"`
	CoverURL           *string    `json:"coverUrl"`
	Status             string     `json:"status"`
	PublishedAt        *time.Time `json:"publishedAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	Category           *Category  `json:"category"`
	Tags               []Tag      `json:"tags"`
	WordCount          uint       `json:"wordCount"`
	ReadingTimeMinutes uint       `json:"readingTimeMinutes"`
}

type PostDetail struct {
	PostSummary
	ContentMarkdown string    `json:"contentMarkdown"`
	PreviousPost    *PostLink `json:"previousPost"`
	NextPost        *PostLink `json:"nextPost"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

type PostPage struct {
	Items      []PostSummary `json:"items"`
	Pagination Pagination    `json:"pagination"`
}

type PublicPostQuery struct {
	Page         int
	PageSize     int
	TagSlug      string
	CategorySlug string
}

type AdminPostQuery struct {
	Page     int
	PageSize int
	Status   string
}

type PostMutation struct {
	Title           string     `json:"title"`
	Slug            string     `json:"slug"`
	Excerpt         string     `json:"excerpt"`
	ContentMarkdown string     `json:"contentMarkdown"`
	CoverURL        *string    `json:"coverUrl"`
	Status          string     `json:"status"`
	PublishedAt     *time.Time `json:"publishedAt"`
	CategoryID      *int64     `json:"categoryId"`
	TagIDs          []int64    `json:"tagIds"`
}

type TaxonomyMutation struct {
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description,omitempty"`
}

type SiteAppearanceMutation struct {
	AvatarURL *string `json:"avatarUrl"`
	BannerURL *string `json:"bannerUrl"`
}
