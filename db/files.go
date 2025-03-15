package db

type File struct {
	Id           int
	FileId       string
	FileUniqueId string
	FileName     string
	FileSize     int
	MessageId    int
	UserId       int
}
