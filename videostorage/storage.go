package videostorage

type Storage interface {
	Download(key string, dest string) error
	Upload(key string, sourceFile string) error
	Delete(key string) error
}
