package app

import "os"

func statFile(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func symlink(src, dst string) error {
	return os.Symlink(src, dst)
}
