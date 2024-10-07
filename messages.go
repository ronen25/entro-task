package main

const (
	NotYetStarted = iota
	Scanning
	ScanDone
)

type ScanMessage struct {
	RepoAuthor string `json:"author"`
	RepoName   string `json:"name"`
	ScanStatus int    `json:"status"`
	LastPage   int    `json:"last_page"`
}

func NewScanMessage(author, name string) ScanMessage {
	return ScanMessage{
		RepoAuthor: author,
		RepoName:   name,
		ScanStatus: NotYetStarted,
		LastPage:   0, // This signals the first (1) page to be scanned
	}
}
