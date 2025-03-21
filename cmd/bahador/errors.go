package main

type EmptyFileNameError struct{}

func (e *EmptyFileNameError) Error() string {
	return "file name is empty"
}

type MaxFileSizeError struct{}

func (e *MaxFileSizeError) Error() string {
	return "file size is bigger than maximum file size"
}

type IncompleteDownloadError struct{}

func (e *IncompleteDownloadError) Error() string {
	return "file download is incomplete"
}

type NonZeroStatusError struct{}

func (e *NonZeroStatusError) Error() string {
	return "non-zero http response status code"
}

var (
	ErrEmptyFileName      = &EmptyFileNameError{}
	ErrMaxFileSize        = &MaxFileSizeError{}
	ErrIncompleteDownload = &IncompleteDownloadError{}
	ErrNonZeroStatusCode  = &NonZeroStatusError{}
)
